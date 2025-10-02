package output

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
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
		return fmt.Errorf("elasticsearch returned status %d", resp.StatusCode)
	}

	// Parse response to check for errors
	var bulkResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&bulkResp); err != nil {
		return fmt.Errorf("failed to parse bulk response: %w", err)
	}

	if errors, ok := bulkResp["errors"].(bool); ok && errors {
		return fmt.Errorf("bulk request had errors")
	}

	return nil
}

func (es *ElasticsearchStrategy) Close() error {
	// Nothing to close for HTTP client
	return nil
}
