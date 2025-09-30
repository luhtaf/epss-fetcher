package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"epss-fetcher-app/config"
	"epss-fetcher-app/orchestrator"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	reset := flag.Bool("reset", false, "Reset checkpoint and start from beginning")
	targetDate := flag.String("date", "", "Target date for incremental update (YYYY-MM-DD), empty for auto-detect")
	incremental := flag.Bool("incremental", false, "Force incremental mode (fetch only new data since last checkpoint)")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Loaded configuration: Strategy=%s, Fetchers=%d, Processors=%d, BulkSize=%d",
		cfg.Strategy, cfg.Workers.Fetchers, cfg.Workers.Processors, cfg.Bulk.Size)

	// Create orchestrator
	orch, err := orchestrator.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create orchestrator: %v", err)
	}
	defer orch.Close()

	// Reset checkpoint if requested
	if *reset {
		log.Println("Resetting checkpoint...")
		if err := os.Remove(cfg.Checkpoint.FilePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: Failed to remove checkpoint file: %v", err)
		}
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down gracefully...", sig)
		cancel()
	}()

	// Run the orchestrator
	log.Println("Starting EPSS fetcher...")
	if err := orch.RunWithMode(ctx, *targetDate, *incremental); err != nil && err != context.Canceled {
		log.Printf("Orchestrator error: %v", err)
		os.Exit(1)
	}

	log.Println("EPSS fetcher completed successfully")
}
