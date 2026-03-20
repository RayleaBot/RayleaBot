package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type Status string

const (
	StatusPending     Status = "pending"
	StatusRunning     Status = "running"
	StatusSucceeded   Status = "succeeded"
	StatusFailed      Status = "failed"
	StatusCancelled   Status = "cancelled"
	StatusInterrupted Status = "interrupted"
)

type ResultSummary struct {
	Summary string         `json:"summary"`
	Details map[string]any `json:"details,omitempty"`
}

type ErrorSummary struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type Snapshot struct {
	TaskID     string         `json:"task_id"`
	TaskType   string         `json:"task_type"`
	Status     Status         `json:"status"`
	Progress   int            `json:"progress,omitempty"`
	Summary    string         `json:"summary"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Result     *ResultSummary `json:"result,omitempty"`
	Error      *ErrorSummary  `json:"error,omitempty"`
}

type Registry struct {
	mu    sync.Mutex
	items map[string]Snapshot
	order []string
}

func NewRegistry() *Registry {
	return &Registry{
		items: map[string]Snapshot{},
		order: []string{},
	}
}

func (r *Registry) List() []Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]Snapshot, 0, len(r.order))
	for _, taskID := range r.order {
		result = append(result, r.items[taskID])
	}

	return result
}

func (r *Registry) Get(taskID string) (Snapshot, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	snapshot, ok := r.items[taskID]
	return snapshot, ok
}

// Create creates a new task snapshot with the given type and summary.
// It generates a unique task_id in the format "task_{16-byte-hex}" using crypto/rand.
func (r *Registry) Create(taskType string, summary string) (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate task id: %w", err)
	}
	taskID := "task_" + hex.EncodeToString(buf[:])

	r.mu.Lock()
	defer r.mu.Unlock()

	r.items[taskID] = Snapshot{
		TaskID:   taskID,
		TaskType: taskType,
		Status:   StatusPending,
		Summary:  summary,
	}
	r.order = append(r.order, taskID)

	return taskID, nil
}
