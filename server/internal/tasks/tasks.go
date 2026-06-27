package tasks

import (
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
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

type Update struct {
	Status     *Status
	Progress   *int
	Summary    *string
	StartedAt  *time.Time
	FinishedAt *time.Time
	Result     *ResultSummary
	Error      *ErrorSummary
}

type LogSink interface {
	Append(logging.Summary)
}

type Registry struct {
	mu               sync.RWMutex
	items            map[string]Snapshot
	order            []string
	nextSubscriberID uint64
	subscribers      map[uint64]chan Snapshot
	repo             Repository
	logs             LogSink
}

func NewRegistry() *Registry {
	return &Registry{
		items:       map[string]Snapshot{},
		order:       []string{},
		subscribers: map[uint64]chan Snapshot{},
	}
}
