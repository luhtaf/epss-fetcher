package models

import "time"

// EPSSResponse represents the response from EPSS API
type EPSSResponse struct {
	Status     string     `json:"status"`
	StatusCode int        `json:"status-code"`
	Version    string     `json:"version"`
	Access     string     `json:"access"`
	Total      int        `json:"total"`
	Offset     int        `json:"offset"`
	Limit      int        `json:"limit"`
	Data       []EPSSData `json:"data"`
}

// EPSSData represents individual EPSS data point
type EPSSData struct {
	CVE        string    `json:"cve"`
	EPSS       string    `json:"epss"`
	Percentile string    `json:"percentile"`
	Date       string    `json:"date"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
}

// Checkpoint represents the current progress state
type Checkpoint struct {
	Offset        int       `json:"offset"`
	Total         int       `json:"total"`
	Processed     int       `json:"processed"`
	LastUpdated   time.Time `json:"last_updated"`
	StartTime     time.Time `json:"start_time"`
	LastDataDate  string    `json:"last_data_date,omitempty"`  // YYYY-MM-DD format
	Mode          string    `json:"mode"`                       // "full" or "incremental"
	FailedRecords []string  `json:"failed_records,omitempty"`
}

// JobBatch represents a batch of work for Stage 2
type JobBatch struct {
	Data      []EPSSData
	BatchID   int
	Timestamp time.Time
}

// ProcessingStats represents runtime statistics
type ProcessingStats struct {
	StartTime     time.Time
	EndTime       time.Time
	TotalRecords  int
	Processed     int
	Failed        int
	SuccessRate   float64
	Duration      time.Duration
	RecordsPerSec float64
}