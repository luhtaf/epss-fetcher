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
	client        *client.EPSSClient
	config        *config.Config
	outputChan    chan []models.EPSSData
	errorChan     chan error
	completionChan chan bool
	fetchDate     string // Empty for full mode, YYYY-MM-DD for incremental
}

func NewFetcherPool(client *client.EPSSClient, cfg *config.Config) *FetcherPool {
	return &FetcherPool{
		client:        client,
		config:        cfg,
		outputChan:    make(chan []models.EPSSData, cfg.Workers.Fetchers*2), // Buffer for smooth flow
		errorChan:     make(chan error, cfg.Workers.Fetchers),
		completionChan: make(chan bool, 1), // Buffer for completion signal
		fetchDate:     "", // Full mode
	}
}

func NewFetcherPoolWithDate(client *client.EPSSClient, cfg *config.Config, date string) *FetcherPool {
	return &FetcherPool{
		client:        client,
		config:        cfg,
		outputChan:    make(chan []models.EPSSData, cfg.Workers.Fetchers*2), // Buffer for smooth flow
		errorChan:     make(chan error, cfg.Workers.Fetchers),
		completionChan: make(chan bool, 1), // Buffer for completion signal
		fetchDate:     date, // Incremental mode
	}
}

func (fp *FetcherPool) Start(ctx context.Context, offsetChan <-chan int, totalRecords int) (<-chan []models.EPSSData, <-chan error, <-chan bool) {
	// Start fetcher workers
	for i := 0; i < fp.config.Workers.Fetchers; i++ {
		go fp.fetchWorker(ctx, i, offsetChan)
	}

	return fp.outputChan, fp.errorChan, fp.completionChan
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
			} else {
				// Empty data received - API has no more records
				log.Printf("Worker %d: Received empty data at offset %d, API exhausted - signaling completion", workerID, offset)
				
				// Send completion signal (non-blocking)
				select {
				case fp.completionChan <- true:
					log.Printf("Worker %d: Completion signal sent", workerID)
				default:
					// Channel might be full, that's ok - another worker already signaled
				}
				
				// Exit this worker since API is exhausted
				return
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

		var resp *models.EPSSResponse
		var err error

		if fp.fetchDate != "" {
			// Date-based incremental fetch
			resp, err = fp.client.FetchEPSSDataByDate(ctx, fp.fetchDate, offset, fp.config.API.PageSize)
		} else {
			// Full fetch
			resp, err = fp.client.FetchEPSSData(ctx, offset, fp.config.API.PageSize)
		}

		if err != nil {
			lastErr = err
			continue
		}

		// Check if we've reached the end of available data
		if len(resp.Data) == 0 || offset >= resp.Total {
			log.Printf("Reached end of data: offset=%d, total=%d, received=%d records",
				offset, resp.Total, len(resp.Data))
			return resp.Data, nil // Return empty data to signal completion
		}

		return resp.Data, nil
	}

	return nil, fmt.Errorf("exhausted retries for offset %d: %w", offset, lastErr)
}

func (fp *FetcherPool) Close() {
	close(fp.outputChan)
	close(fp.errorChan)
	close(fp.completionChan)
}
