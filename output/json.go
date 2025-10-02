package output

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luhtaf/epss-fetcher/config"
	"github.com/luhtaf/epss-fetcher/models"
)

type JSONStrategy struct {
	config *config.JSONConfig
}

func NewJSONStrategy(cfg *config.JSONConfig) (*JSONStrategy, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &JSONStrategy{
		config: cfg,
	}, nil
}

func (js *JSONStrategy) Write(ctx context.Context, batch []models.EPSSData, batchID int) error {
	if len(batch) == 0 {
		return nil
	}

	// Generate filename
	filename := fmt.Sprintf(js.config.FilePattern, batchID)
	filePath := filepath.Join(js.config.OutputDir, filename)

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	// Write data based on format
	switch js.config.Format {
	case "array":
		return js.writeArray(file, batch)
	case "ndjson":
		return js.writeNDJSON(file, batch)
	default:
		return fmt.Errorf("unknown JSON format: %s", js.config.Format)
	}
}

func (js *JSONStrategy) writeArray(file *os.File, batch []models.EPSSData) error {
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(batch)
}

func (js *JSONStrategy) writeNDJSON(file *os.File, batch []models.EPSSData) error {
	encoder := json.NewEncoder(file)
	for _, record := range batch {
		if err := encoder.Encode(record); err != nil {
			return fmt.Errorf("failed to encode record: %w", err)
		}
	}
	return nil
}

func (js *JSONStrategy) Close() error {
	// Nothing to close for file-based strategy
	return nil
}
