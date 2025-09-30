package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"epss-fetcher/models"
)

type Manager struct {
	filePath   string
	checkpoint *models.Checkpoint
	mu         sync.RWMutex
	enabled    bool
}

func NewManager(filePath string, enabled bool) *Manager {
	return &Manager{
		filePath: filePath,
		enabled:  enabled,
		checkpoint: &models.Checkpoint{
			StartTime: time.Now(),
		},
	}
}

func (m *Manager) Load() error {
	if !m.enabled {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if os.IsNotExist(err) {
		// File doesn't exist, start fresh
		m.checkpoint = &models.Checkpoint{
			StartTime: time.Now(),
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read checkpoint file: %w", err)
	}

	if err := json.Unmarshal(data, m.checkpoint); err != nil {
		return fmt.Errorf("failed to parse checkpoint: %w", err)
	}

	return nil
}

func (m *Manager) Save() error {
	if !m.enabled {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	m.checkpoint.LastUpdated = time.Now()

	data, err := json.MarshalIndent(m.checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint file: %w", err)
	}

	return nil
}

func (m *Manager) UpdateProgress(offset, total, processed int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkpoint.Offset = offset
	m.checkpoint.Total = total
	m.checkpoint.Processed = processed
}

func (m *Manager) UpdateMode(mode, date string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkpoint.Mode = mode
	m.checkpoint.LastDataDate = date
}

func (m *Manager) AddFailedRecord(record string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkpoint.FailedRecords = append(m.checkpoint.FailedRecords, record)
}

func (m *Manager) GetCheckpoint() models.Checkpoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return *m.checkpoint
}

func (m *Manager) GetOffset() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.checkpoint.Offset
}

func (m *Manager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkpoint = &models.Checkpoint{
		StartTime: time.Now(),
	}
}
