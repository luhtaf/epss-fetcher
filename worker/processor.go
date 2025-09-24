package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/luhtaf/epss-fetcher/config"
	"github.com/luhtaf/epss-fetcher/models"
	"github.com/luhtaf/epss-fetcher/output"
)

type ProcessorPool struct {
	config     *config.Config
	strategy   output.Strategy
	batchCount int
	mu         sync.Mutex
	errorChan  chan error
}

func NewProcessorPool(cfg *config.Config, strategy output.Strategy) *ProcessorPool {
	return &ProcessorPool{
		config:    cfg,
		strategy:  strategy,
		errorChan: make(chan error, cfg.Workers.Processors),
	}
}

func (pp *ProcessorPool) Start(ctx context.Context, inputChan <-chan []models.EPSSData) <-chan error {
	// Start processor workers
	for i := 0; i < pp.config.Workers.Processors; i++ {
		go pp.processWorker(ctx, i, inputChan)
	}

	return pp.errorChan
}

func (pp *ProcessorPool) processWorker(ctx context.Context, workerID int, inputChan <-chan []models.EPSSData) {
	defer func() {
		if r := recover(); r != nil {
			pp.errorChan <- fmt.Errorf("processor %d panicked: %v", workerID, r)
		}
	}()

	buffer := make([]models.EPSSData, 0, pp.config.Bulk.Size)
	flushTicker := time.NewTicker(pp.config.Bulk.Timeout)
	defer flushTicker.Stop()

	for {
		select {
		case batch, ok := <-inputChan:
			if !ok {
				// Channel closed, flush remaining data
				if len(buffer) > 0 {
					pp.flushBuffer(ctx, workerID, buffer)
				}
				return
			}

			// Add batch to buffer
			buffer = append(buffer, batch...)

			// Check if buffer is full
			if len(buffer) >= pp.config.Bulk.Size {
				// Flush full buffer
				toFlush := buffer[:pp.config.Bulk.Size]
				buffer = buffer[pp.config.Bulk.Size:]

				pp.flushBuffer(ctx, workerID, toFlush)
				flushTicker.Reset(pp.config.Bulk.Timeout) // Reset timeout
			}

		case <-flushTicker.C:
			// Timeout reached, flush whatever we have
			if len(buffer) > 0 {
				pp.flushBuffer(ctx, workerID, buffer)
				buffer = buffer[:0] // Clear buffer
			}

		case <-ctx.Done():
			// Context cancelled, flush remaining data
			if len(buffer) > 0 {
				pp.flushBuffer(ctx, workerID, buffer)
			}
			return
		}
	}
}

func (pp *ProcessorPool) flushBuffer(ctx context.Context, workerID int, buffer []models.EPSSData) {
	if len(buffer) == 0 {
		return
	}

	// Get unique batch ID
	pp.mu.Lock()
	pp.batchCount++
	batchID := pp.batchCount
	pp.mu.Unlock()

	log.Printf("Processor %d: Flushing batch %d with %d records", workerID, batchID, len(buffer))

	// Write using strategy with retry
	var lastErr error
	for attempt := 0; attempt <= pp.config.Retry.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			delay := time.Duration(float64(pp.config.Retry.Delay) *
				(pp.config.Retry.Backoff * float64(attempt)))

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return
			}
		}

		if err := pp.strategy.Write(ctx, buffer, batchID); err != nil {
			lastErr = err
			log.Printf("Processor %d: Attempt %d failed for batch %d: %v", workerID, attempt+1, batchID, err)
			continue
		}

		// Success
		return
	}

	// All retries failed
	pp.errorChan <- fmt.Errorf("processor %d: failed to write batch %d after %d retries: %w",
		workerID, batchID, pp.config.Retry.MaxRetries, lastErr)
}

func (pp *ProcessorPool) Close() {
	if pp.strategy != nil {
		pp.strategy.Close()
	}
	close(pp.errorChan)
}
