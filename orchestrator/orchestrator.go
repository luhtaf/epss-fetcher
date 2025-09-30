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
	return o.RunWithMode(ctx, "", false)
}

func (o *Orchestrator) RunWithMode(ctx context.Context, targetDate string, forceIncremental bool) error {
	log.Println("Starting EPSS fetcher...")

	// Load existing checkpoint to determine mode
	checkpoint := o.checkpointMgr.GetCheckpoint()
	
	var (
		mode         string
		fetchDate    string
		totalRecords int
		startOffset  int
		err          error
	)

	// Smart mode detection
	if targetDate != "" {
		// Explicit date provided
		mode = "incremental"
		fetchDate = targetDate
		log.Printf("Running in incremental mode for date: %s", fetchDate)
		
		totalRecords, err = o.client.GetTotalRecordsForDate(ctx, fetchDate)
		if err != nil {
			return fmt.Errorf("failed to get total records for date %s: %w", fetchDate, err)
		}
		startOffset = 0 // Always start from 0 for date-based queries
		
	} else if forceIncremental && checkpoint.LastDataDate != "" {
		// Incremental mode: fetch from last checkpoint date to today
		mode = "incremental"
		today := time.Now().Format("2006-01-02")
		
		if checkpoint.LastDataDate == today {
			log.Printf("Data already up to date (last update: %s)", checkpoint.LastDataDate)
			return nil
		}
		
		fetchDate = today
		log.Printf("Running incremental update from %s to %s", checkpoint.LastDataDate, today)
		
		totalRecords, err = o.client.GetTotalRecordsForDate(ctx, fetchDate)
		if err != nil {
			log.Printf("Failed to get records for today (%s), falling back to full mode", today)
			mode = "full"
		} else {
			startOffset = 0
		}
		
	} else if checkpoint.LastDataDate == "" || checkpoint.Mode == "" {
		// No checkpoint or old checkpoint format: full mode
		mode = "full"
		fetchDate = ""
		log.Println("No valid checkpoint found, running full mode")
		
		totalRecords, err = o.client.GetTotalRecords(ctx)
		if err != nil {
			return fmt.Errorf("failed to get total records: %w", err)
		}
		startOffset = checkpoint.Offset
		
	} else {
		// Continue from existing checkpoint
		mode = checkpoint.Mode
		fetchDate = checkpoint.LastDataDate
		if mode == "incremental" && fetchDate != "" {
			totalRecords, err = o.client.GetTotalRecordsForDate(ctx, fetchDate)
			if err != nil {
				log.Printf("Failed to resume incremental mode, switching to full mode")
				mode = "full"
				fetchDate = ""
				totalRecords, err = o.client.GetTotalRecords(ctx)
				if err != nil {
					return fmt.Errorf("failed to get total records: %w", err)
				}
			}
		} else {
			totalRecords, err = o.client.GetTotalRecords(ctx)
			if err != nil {
				return fmt.Errorf("failed to get total records: %w", err)
			}
		}
		startOffset = checkpoint.Offset
	}

	o.statsTracker.SetTotal(totalRecords)
	log.Printf("Mode: %s, Date: %s, Total records: %d, Starting offset: %d", 
		mode, fetchDate, totalRecords, startOffset)

	// Update checkpoint with current mode
	o.checkpointMgr.UpdateMode(mode, fetchDate)

	// Create progress bar
	remaining := totalRecords - startOffset
	progressBar := progressbar.DefaultBytes(int64(remaining), "Processing EPSS data")

	// Create offset channel for Stage 1
	offsetChan := make(chan int, o.config.Workers.Fetchers)

	// Start Stage 1: Fetcher workers with date support
	fetcherPool := worker.NewFetcherPoolWithDate(o.client, o.config, fetchDate)
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
