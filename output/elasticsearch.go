package output

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/luhtaf/epss-fetcher/config"
	"github.com/luhtaf/epss-fetcher/models"
)

type ElasticsearchStrategy struct {
	client *http.Client
	config *config.ElasticsearchConfig
	hosts  []string
}

// bulkResponse is the subset of the _bulk reply we need to tell which
// individual documents failed. Each item is a single-key map whose key is the
// action name ("index", "update", ...).
type bulkResponse struct {
	Errors bool `json:"errors"`
	Items  []map[string]struct {
		Index  string `json:"_index"`
		ID     string `json:"_id"`
		Status int    `json:"status"`
		Error  *struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		} `json:"error"`
	} `json:"items"`
}

func NewElasticsearchStrategy(cfg *config.ElasticsearchConfig) (*ElasticsearchStrategy, error) {
	// Create custom transport with TLS config
	transport := &http.Transport{}

	if cfg.SkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	} else if cfg.CACertPath != "" {
		// Load custom CA certificate
		caCert, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		transport.TLSClientConfig = &tls.Config{
			RootCAs: caCertPool,
		}
	}

	client := &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}

	return &ElasticsearchStrategy{
		client: client,
		config: cfg,
		hosts:  cfg.Hosts,
	}, nil
}

func (es *ElasticsearchStrategy) Write(ctx context.Context, batch []models.EPSSData, batchID int) error {
	if len(batch) == 0 {
		return nil
	}

	primary, err := es.buildPrimaryBody(batch)
	if err != nil {
		return err
	}
	if err := es.bulkRequest(ctx, primary); err != nil {
		return fmt.Errorf("index %s: %w", es.config.Index, err)
	}

	if !es.config.Enrich.Enabled {
		return nil
	}

	// The enrich write goes out as its own request rather than sharing the
	// bulk above. flushBuffer retries the whole batch on error, so a mixed
	// body would drag the already-successful primary writes through a rewrite
	// -- and fail_on_error:false would be impossible to honour.
	enrich, err := es.buildEnrichBody(batch)
	if err != nil {
		return err
	}
	if err := es.bulkRequest(ctx, enrich); err != nil {
		if es.config.Enrich.FailOnError {
			return fmt.Errorf("enrich %s: %w", es.config.Enrich.Index, err)
		}
		log.Printf("WARN batch %d: enrich to %s failed (ignored): %v",
			batchID, es.config.Enrich.Index, err)
	}

	return nil
}

// buildPrimaryBody produces the ndjson for the main index: a full document
// replace keyed on the CVE id. This is the original, unchanged behaviour.
func (es *ElasticsearchStrategy) buildPrimaryBody(batch []models.EPSSData) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	for _, record := range batch {
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": es.config.Index,
				"_id":    record.CVE,
			},
		}
		actionBytes, err := json.Marshal(action)
		if err != nil {
			return nil, fmt.Errorf("failed to encode bulk action for %s: %w", record.CVE, err)
		}
		buf.Write(actionBytes)
		buf.WriteByte('\n')

		docBytes, err := json.Marshal(record)
		if err != nil {
			return nil, fmt.Errorf("failed to encode document %s: %w", record.CVE, err)
		}
		buf.Write(docBytes)
		buf.WriteByte('\n')
	}
	return &buf, nil
}

// buildEnrichBody produces the ndjson for the enrich index. It writes only the
// EPSS fields, as a partial update, so every other field on an existing CVE
// document survives untouched.
func (es *ElasticsearchStrategy) buildEnrichBody(batch []models.EPSSData) (*bytes.Buffer, error) {
	cfg := es.config.Enrich
	prefix := cfg.FieldPrefix

	// "upsert" and "update" are both _bulk "update" actions; they differ only
	// in whether a missing document is created.
	actionName := "update"
	if cfg.Mode == "index" {
		actionName = "index"
	}

	var buf bytes.Buffer
	var unparsable int
	for _, record := range batch {
		action := map[string]interface{}{
			actionName: map[string]interface{}{
				"_index": cfg.Index,
				"_id":    record.CVE,
			},
		}
		actionBytes, err := json.Marshal(action)
		if err != nil {
			return nil, fmt.Errorf("failed to encode enrich action for %s: %w", record.CVE, err)
		}
		buf.Write(actionBytes)
		buf.WriteByte('\n')

		doc := map[string]interface{}{}
		if cfg.Numeric {
			// The EPSS API sends these as JSON strings. Storing them as floats
			// is what makes range queries such as epss > 0.5 work.
			epss, errEPSS := strconv.ParseFloat(record.EPSS, 64)
			pct, errPct := strconv.ParseFloat(record.Percentile, 64)
			if errEPSS != nil || errPct != nil {
				unparsable++
			}
			if errEPSS == nil {
				doc[prefix+"epss"] = epss
			}
			if errPct == nil {
				doc[prefix+"percentile"] = pct
			}
		} else {
			doc[prefix+"epss"] = record.EPSS
			doc[prefix+"percentile"] = record.Percentile
		}
		doc[prefix+"epss_date"] = record.Date

		var payload interface{} = doc
		if actionName == "update" {
			p := map[string]interface{}{"doc": doc}
			if cfg.Mode == "upsert" {
				// Without this, a CVE not yet present in the enrich index
				// returns a 404 per item and fails the whole bulk.
				p["doc_as_upsert"] = true
			}
			payload = p
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to encode enrich document %s: %w", record.CVE, err)
		}
		buf.Write(payloadBytes)
		buf.WriteByte('\n')
	}

	if unparsable > 0 {
		log.Printf("WARN enrich: %d/%d records had an unparsable epss/percentile value", unparsable, len(batch))
	}

	return &buf, nil
}

// bulkRequest posts one ndjson body to _bulk and reports per-item failures.
func (es *ElasticsearchStrategy) bulkRequest(ctx context.Context, buf *bytes.Buffer) error {
	url := fmt.Sprintf("%s/_bulk", es.hosts[0])
	req, err := http.NewRequestWithContext(ctx, "POST", url, buf)
	if err != nil {
		return fmt.Errorf("failed to create bulk request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("User-Agent", "epss-fetcher/1.0")

	// Add authentication if configured
	if es.config.Username != "" && es.config.Password != "" {
		req.SetBasicAuth(es.config.Username, es.config.Password)
	}

	resp, err := es.client.Do(req)
	if err != nil {
		return fmt.Errorf("bulk request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("elasticsearch returned status %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}

	var br bulkResponse
	if err := json.NewDecoder(resp.Body).Decode(&br); err != nil {
		return fmt.Errorf("failed to parse bulk response: %w", err)
	}

	if !br.Errors {
		return nil
	}

	// errors:true means at least one item failed -- not necessarily all of them.
	// Report how many and why, so the log says something actionable.
	var failed int
	var sample string
	for _, item := range br.Items {
		for _, res := range item {
			if res.Error == nil {
				continue
			}
			failed++
			if sample == "" {
				sample = fmt.Sprintf("%s (%s: %s)", res.ID, res.Error.Type, res.Error.Reason)
			}
		}
	}

	if failed > 0 {
		return fmt.Errorf("bulk: %d/%d documents failed, first: %s", failed, len(br.Items), sample)
	}

	return nil
}

func (es *ElasticsearchStrategy) Close() error {
	// Nothing to close for HTTP client
	return nil
}
