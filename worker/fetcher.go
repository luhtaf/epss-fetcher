package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/luhtaf/epss-fetcher/client"
	"github.com/luhtaf/epss-fetcher/config"
	"github.com/luhtaf/epss-fetcher/models"
)

type FetcherPool struct {
	client         *client.EPSSClient
	config         *config.Config
	outputChan     chan []models.EPSSData
	errorChan      chan error
	completionChan chan bool
	fetchDate      string // Empty for full mode, YYYY-MM-DD for incremental
	wg             sync.WaitGroup
	watcherWg      sync.WaitGroup // the completion watcher started by Start
}

func NewFetcherPool(client *client.EPSSClient, cfg *config.Config) *FetcherPool {
	return &FetcherPool{
		client:         client,
		config:         cfg,
		outputChan:     make(chan []models.EPSSData, cfg.Workers.Fetchers*2), // Buffer for smooth flow
		errorChan:      make(chan error, cfg.Workers.Fetchers),
		completionChan: make(chan bool, 1), // Buffer for completion signal
		fetchDate:      "",                 // Full mode
	}
}

func NewFetcherPoolWithDate(client *client.EPSSClient, cfg *config.Config, date string) *FetcherPool {
	return &FetcherPool{
		client:         client,
		config:         cfg,
		outputChan:     make(chan []models.EPSSData, cfg.Workers.Fetchers*2), // Buffer for smooth flow
		errorChan:      make(chan error, cfg.Workers.Fetchers),
		completionChan: make(chan bool, 1), // Buffer for completion signal
		fetchDate:      date,               // Incremental mode
	}
}

func (fp *FetcherPool) Start(ctx context.Context, offsetChan <-chan int, totalRecords int) (<-chan []models.EPSSData, <-chan error, <-chan bool) {
	// Start fetcher workers
	for i := 0; i < fp.config.Workers.Fetchers; i++ {
		fp.wg.Add(1)
		go fp.fetchWorker(ctx, i, offsetChan)
	}

	// Watcher: once every fetcher worker has exited (drained, errored, or
	// hit an empty page), signal completion so the orchestrator never blocks
	// forever. Wait tracks this goroutine too, so Close cannot close
	// completionChan while the send below is still in flight.
	fp.watcherWg.Add(1)
	go func() {
		defer fp.watcherWg.Done()
		fp.wg.Wait()
		select {
		case fp.completionChan <- true:
		default:
			// Buffer already holds a signal from a worker -- nothing to do.
		}
	}()

	return fp.outputChan, fp.errorChan, fp.completionChan
}

func (fp *FetcherPool) fetchWorker(ctx context.Context, workerID int, offsetChan <-chan int) {
	defer fp.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("worker %d panicked: %v", workerID, r)
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
				select {
				case fp.errorChan <- fmt.Errorf("worker %d failed at offset %d: %w", workerID, offset, err):
				case <-ctx.Done():
					return
				}
				continue
			}

			if len(data) > 0 {
				// Blocking send: if outputChan is full the fetcher waits for the
				// processors to catch up. That backpressure is intentional -- never
				// add a default clause here, it silently drops batches.
				select {
				case fp.outputChan <- data:
					// Data sent successfully
				case <-ctx.Done():
					return
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

// Wait blocks until every fetch worker and the completion watcher have exited.
// Call it before Close so nothing is still writing to the channels when they
// are closed.
func (fp *FetcherPool) Wait() {
	fp.wg.Wait()
	fp.watcherWg.Wait()
}

func (fp *FetcherPool) Close() {
	close(fp.outputChan)
	close(fp.errorChan)
	close(fp.completionChan)
}
