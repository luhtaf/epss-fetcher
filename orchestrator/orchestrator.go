package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/schollz/progressbar/v3"

	"github.com/luhtaf/epss-fetcher/checkpoint"
	"github.com/luhtaf/epss-fetcher/client"
	"github.com/luhtaf/epss-fetcher/config"
	"github.com/luhtaf/epss-fetcher/output"
	"github.com/luhtaf/epss-fetcher/stats"
	"github.com/luhtaf/epss-fetcher/worker"
)

type Orchestrator struct {
	config         *config.Config
	client         *client.EPSSClient
	checkpointMgr  *checkpoint.Manager
	statsTracker   *stats.Tracker
	outputStrategy output.Strategy
}

func New(cfg *config.Config) (*Orchestrator, error) {
	// Initialize EPSS client
	epssClient := client.NewEPSSClient(&cfg.API)

	// Initialize checkpoint manager
	checkpointMgr := checkpoint.NewManager(cfg.Checkpoint.FilePath, cfg.Checkpoint.Enabled)
	if err := checkpointMgr.Load(); err != nil {
		return nil, fmt.Errorf("failed to load checkpoint: %w", err)
	}

	// Initialize stats tracker
	statsTracker := stats.NewTracker(cfg.Logging.OutputFile)

	// Initialize output strategy
	strategy, err := output.NewStrategy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create output strategy: %w", err)
	}

	return &Orchestrator{
		config:         cfg,
		client:         epssClient,
		checkpointMgr:  checkpointMgr,
		statsTracker:   statsTracker,
		outputStrategy: strategy,
	}, nil
}

func (o *Orchestrator) Run(ctx context.Context) error {
	log.Println("Starting EPSS fetcher...")

	// Get total records
	totalRecords, err := o.client.GetTotalRecords(ctx)
	if err != nil {
		return fmt.Errorf("failed to get total records: %w", err)
	}

	o.statsTracker.SetTotal(totalRecords)
	log.Printf("Total records to process: %d", totalRecords)

	// Get starting offset from checkpoint
	startOffset := o.checkpointMgr.GetOffset()
	log.Printf("Starting from offset: %d", startOffset)

	// Create progress bar
	remaining := totalRecords - startOffset
	progressBar := progressbar.DefaultBytes(int64(remaining), "Processing EPSS data")

	// Create offset channel for Stage 1
	offsetChan := make(chan int, o.config.Workers.Fetchers)

	// Start Stage 1: Fetcher workers
	fetcherPool := worker.NewFetcherPool(o.client, o.config)
	dataChan, fetchErrorChan := fetcherPool.Start(ctx, offsetChan, totalRecords)

	// Start Stage 2: Processor workers
	processorPool := worker.NewProcessorPool(o.config, o.outputStrategy)
	processErrorChan := processorPool.Start(ctx, dataChan)

	// Start offset generator
	go o.generateOffsets(ctx, offsetChan, startOffset, totalRecords)

	// Start error handlers
	go o.handleErrors(ctx, fetchErrorChan, processErrorChan)

	// Monitor progress
	go o.monitorProgress(ctx, progressBar, startOffset, totalRecords)

	// Wait for completion or cancellation
	<-ctx.Done()

	// Cleanup
	fetcherPool.Close()
	processorPool.Close()

	// Save final checkpoint and stats
	if err := o.checkpointMgr.Save(); err != nil {
		log.Printf("Failed to save checkpoint: %v", err)
	}

	if err := o.statsTracker.SaveSummary(); err != nil {
		log.Printf("Failed to save summary: %v", err)
	}

	o.statsTracker.PrintSummary()

	return ctx.Err()
}

func (o *Orchestrator) generateOffsets(ctx context.Context, offsetChan chan<- int, startOffset, totalRecords int) {
	defer close(offsetChan)

	for offset := startOffset; offset < totalRecords; offset += o.config.API.PageSize {
		select {
		case offsetChan <- offset:
		case <-ctx.Done():
			return
		}
	}
}

func (o *Orchestrator) handleErrors(ctx context.Context, fetchErrorChan, processErrorChan <-chan error) {
	for {
		select {
		case err := <-fetchErrorChan:
			if err != nil {
				log.Printf("Fetch error: %v", err)
				o.statsTracker.IncrementFailed(o.config.API.PageSize)
			}
		case err := <-processErrorChan:
			if err != nil {
				log.Printf("Process error: %v", err)
				o.statsTracker.IncrementFailed(o.config.Bulk.Size)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (o *Orchestrator) monitorProgress(ctx context.Context, progressBar *progressbar.ProgressBar, startOffset, totalRecords int) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastProcessed := 0
	for {
		select {
		case <-ticker.C:
			stats := o.statsTracker.GetStats()
			processed := stats.Processed

			// Update progress bar
			if processed > lastProcessed {
				progressBar.Add(processed - lastProcessed)
				lastProcessed = processed
			}

			// Update checkpoint
			currentOffset := startOffset + processed
			o.checkpointMgr.UpdateProgress(currentOffset, totalRecords, processed)

			// Save checkpoint periodically
			if err := o.checkpointMgr.Save(); err != nil {
				log.Printf("Failed to save checkpoint: %v", err)
			}

		case <-ctx.Done():
			return
		}
	}
}

func (o *Orchestrator) Close() error {
	if o.outputStrategy != nil {
		return o.outputStrategy.Close()
	}
	return nil
}
