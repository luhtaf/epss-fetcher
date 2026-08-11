package output

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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

	// Build bulk request body
	var buf bytes.Buffer
	for _, record := range batch {
		// Index action
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": es.config.Index,
				"_id":    record.CVE,
			},
		}
		actionBytes, _ := json.Marshal(action)
		buf.Write(actionBytes)
		buf.WriteByte('\n')

		// Document
		docBytes, _ := json.Marshal(record)
		buf.Write(docBytes)
		buf.WriteByte('\n')
	}

	// Send bulk request
	url := fmt.Sprintf("%s/_bulk", es.hosts[0])
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
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
