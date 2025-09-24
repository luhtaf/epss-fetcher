package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/luhtaf/epss-fetcher/models"
)

type Tracker struct {
	stats      *models.ProcessingStats
	mu         sync.RWMutex
	outputFile string
}

func NewTracker(outputFile string) *Tracker {
	return &Tracker{
		stats: &models.ProcessingStats{
			StartTime: time.Now(),
		},
		outputFile: outputFile,
	}
}

func (t *Tracker) IncrementProcessed(count int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats.Processed += count
}

func (t *Tracker) IncrementFailed(count int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats.Failed += count
}

func (t *Tracker) SetTotal(total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stats.TotalRecords = total
}

func (t *Tracker) GetStats() models.ProcessingStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := *t.stats
	stats.EndTime = time.Now()
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	if stats.TotalRecords > 0 {
		stats.SuccessRate = float64(stats.Processed) / float64(stats.TotalRecords) * 100
	}

	if stats.Duration.Seconds() > 0 {
		stats.RecordsPerSec = float64(stats.Processed) / stats.Duration.Seconds()
	}

	return stats
}

func (t *Tracker) SaveSummary() error {
	if t.outputFile == "" {
		return nil
	}

	stats := t.GetStats()

	// Create summary text
	summary := fmt.Sprintf(`EPSS Fetcher - Processing Summary
=================================
Start Time: %s
End Time: %s
Duration: %s
Total Records: %d
Processed: %d
Failed: %d
Success Rate: %.2f%%
Records/sec: %.2f
`,
		stats.StartTime.Format(time.RFC3339),
		stats.EndTime.Format(time.RFC3339),
		stats.Duration.String(),
		stats.TotalRecords,
		stats.Processed,
		stats.Failed,
		stats.SuccessRate,
		stats.RecordsPerSec,
	)

	// Write to file
	if err := os.WriteFile(t.outputFile, []byte(summary), 0644); err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}

	// Also save as JSON
	jsonFile := t.outputFile + ".json"
	jsonData, _ := json.MarshalIndent(stats, "", "  ")
	if err := os.WriteFile(jsonFile, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write JSON summary: %w", err)
	}

	return nil
}

func (t *Tracker) PrintSummary() {
	stats := t.GetStats()
	fmt.Printf("\n=== Processing Summary ===\n")
	fmt.Printf("Duration: %s\n", stats.Duration.String())
	fmt.Printf("Total: %d, Processed: %d, Failed: %d\n",
		stats.TotalRecords, stats.Processed, stats.Failed)
	fmt.Printf("Success Rate: %.2f%%, Records/sec: %.2f\n",
		stats.SuccessRate, stats.RecordsPerSec)
}
