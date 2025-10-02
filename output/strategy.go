package output

import (
	"context"
	"fmt"

	"github.com/luhtaf/epss-fetcher/config"
	"github.com/luhtaf/epss-fetcher/models"
)

// Strategy defines the interface for output strategies
type Strategy interface {
	Write(ctx context.Context, batch []models.EPSSData, batchID int) error
	Close() error
}

// Factory creates output strategies based on configuration
func NewStrategy(cfg *config.Config) (Strategy, error) {
	switch cfg.Strategy {
	case "elasticsearch":
		return NewElasticsearchStrategy(&cfg.Elastic)
	case "json":
		return NewJSONStrategy(&cfg.JSON)
	default:
		return nil, fmt.Errorf("unknown strategy: %s", cfg.Strategy)
	}
}
