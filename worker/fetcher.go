package worker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/luhtaf/epss-fetcher/client"
	"github.com/luhtaf/epss-fetcher/config"
	"github.com/luhtaf/epss-fetcher/models"
)

type FetcherPool struct {
	client     *client.EPSSClient
	config     *config.Config
	outputChan chan []models.EPSSData
	errorChan  chan error
}

func NewFetcherPool(client *client.EPSSClient, cfg *config.Config) *FetcherPool {
	return &FetcherPool{
		client:     client,
		config:     cfg,
		outputChan: make(chan []models.EPSSData, cfg.Workers.Fetchers*2), // Buffer for smooth flow
		errorChan:  make(chan error, cfg.Workers.Fetchers),
	}
}

func (fp *FetcherPool) Start(ctx context.Context, offsetChan <-chan int, totalRecords int) (<-chan []models.EPSSData, <-chan error) {
	// Start fetcher workers
	for i := 0; i < fp.config.Workers.Fetchers; i++ {
		go fp.fetchWorker(ctx, i, offsetChan)
	}

	return fp.outputChan, fp.errorChan
}

func (fp *FetcherPool) fetchWorker(ctx context.Context, workerID int, offsetChan <-chan int) {
	defer func() {
		if r := recover(); r != nil {
			fp.errorChan <- fmt.Errorf("worker %d panicked: %v", workerID, r)
		}
	}()

	for {
		select {
		case offset, ok := <-offsetChan:
			if !ok {
				// Channel closed, exit worker
				return
			}

			// Fetch data with retry logic
			data, err := fp.fetchWithRetry(ctx, offset)
			if err != nil {
				log.Printf("Worker %d: Failed to fetch offset %d: %v", workerID, offset, err)
				fp.errorChan <- fmt.Errorf("worker %d failed at offset %d: %w", workerID, offset, err)
				continue
			}

			if len(data) > 0 {
				select {
				case fp.outputChan <- data:
					// Data sent successfully
				case <-ctx.Done():
					return
				default:
					// Channel might be closed, try non-blocking send
					select {
					case fp.outputChan <- data:
					default:
						log.Printf("Worker %d: Output channel closed, dropping batch", workerID)
						return
					}
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

func (fp *FetcherPool) fetchWithRetry(ctx context.Context, offset int) ([]models.EPSSData, error) {
	var lastErr error

	for attempt := 0; attempt <= fp.config.Retry.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			delay := time.Duration(float64(fp.config.Retry.Delay) *
				(fp.config.Retry.Backoff * float64(attempt)))

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := fp.client.FetchEPSSData(ctx, offset, fp.config.API.PageSize)
		if err != nil {
			lastErr = err
			continue
		}

		return resp.Data, nil
	}

	return nil, fmt.Errorf("exhausted retries for offset %d: %w", offset, lastErr)
}

func (fp *FetcherPool) Close() {
	close(fp.outputChan)
	close(fp.errorChan)
}
