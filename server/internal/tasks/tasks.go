package tasks

import (
	"context"
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

type Update struct {
	Status     *Status
	Progress   *int
	Summary    *string
	StartedAt  *time.Time
	FinishedAt *time.Time
	Result     *ResultSummary
	Error      *ErrorSummary
}

type Registry struct {
	mu               sync.RWMutex
	items            map[string]Snapshot
	order            []string
	nextSubscriberID uint64
	subscribers      map[uint64]chan Snapshot
	repo             Repository
}

func NewRegistry() *Registry {
	return &Registry{
		items:       map[string]Snapshot{},
		order:       []string{},
		subscribers: map[uint64]chan Snapshot{},
	}
}

// SetRepository attaches a persistence backend. When set, every Create and
// Update call will also persist the snapshot to the repository.
func (r *Registry) SetRepository(repo Repository) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.repo = repo
}

// Hydrate loads persisted task snapshots into the in-memory registry. It should
// be called once at startup, before the registry is used by other components.
func (r *Registry) Hydrate(ctx context.Context) error {
	r.mu.Lock()
	repo := r.repo
	r.mu.Unlock()

	if repo == nil {
		return nil
	}

	snapshots, err := repo.LoadTasks(ctx)
	if err != nil {
		return fmt.Errorf("hydrate task registry: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, s := range snapshots {
		if _, exists := r.items[s.TaskID]; exists {
			continue
		}
		r.items[s.TaskID] = s
		r.order = append(r.order, s.TaskID)
	}
	return nil
}

func (r *Registry) List() []Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Snapshot, 0, len(r.order))
	for _, taskID := range r.order {
		result = append(result, cloneSnapshot(r.items[taskID]))
	}

	return result
}

func (r *Registry) Get(taskID string) (Snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot, ok := r.items[taskID]
	return cloneSnapshot(snapshot), ok
}

// Create creates a new task snapshot with the given type and summary.
// It generates a unique task_id in the format "task_{16-byte-hex}" using crypto/rand.
func (r *Registry) Create(taskType string, summary string) (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("generate task id: %w", err)
	}
	taskID := "task_" + hex.EncodeToString(buf[:])

	snapshot := Snapshot{
		TaskID:   taskID,
		TaskType: taskType,
		Status:   StatusPending,
		Summary:  summary,
	}

	r.mu.Lock()
	r.items[taskID] = snapshot
	r.order = append(r.order, taskID)
	r.broadcastLocked(snapshot)
	repo := r.repo
	r.mu.Unlock()

	r.persistAsync(repo, snapshot)

	return taskID, nil
}

func (r *Registry) Update(taskID string, update Update) (Snapshot, bool) {
	r.mu.Lock()

	snapshot, ok := r.items[taskID]
	if !ok {
		r.mu.Unlock()
		return Snapshot{}, false
	}

	if update.Status != nil {
		snapshot.Status = *update.Status
	}
	if update.Progress != nil {
		snapshot.Progress = *update.Progress
	}
	if update.Summary != nil {
		snapshot.Summary = *update.Summary
	}
	if update.StartedAt != nil {
		startedAt := (*update.StartedAt).UTC()
		snapshot.StartedAt = &startedAt
	}
	if update.FinishedAt != nil {
		finishedAt := (*update.FinishedAt).UTC()
		snapshot.FinishedAt = &finishedAt
	}
	if update.Result != nil {
		snapshot.Result = cloneResult(update.Result)
	}
	if update.Error != nil {
		snapshot.Error = cloneError(update.Error)
	}

	r.items[taskID] = snapshot
	r.broadcastLocked(snapshot)
	cloned := cloneSnapshot(snapshot)
	repo := r.repo
	r.mu.Unlock()

	r.persistAsync(repo, snapshot)

	return cloned, true
}

// persistAsync saves a snapshot to the repository in a fire-and-forget manner.
// Persistence errors are silently dropped because the in-memory registry is the
// authoritative source during the current process lifetime.
func (r *Registry) persistAsync(repo Repository, snapshot Snapshot) {
	if repo == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = repo.SaveTask(ctx, snapshot)
	}()
}

func (r *Registry) Subscribe(buffer int) (<-chan Snapshot, func()) {
	if buffer <= 0 {
		buffer = 1
	}

	ch := make(chan Snapshot, buffer)

	r.mu.Lock()
	id := r.nextSubscriberID
	r.nextSubscriberID++
	r.subscribers[id] = ch
	r.mu.Unlock()

	return ch, func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		subscriber, ok := r.subscribers[id]
		if !ok {
			return
		}

		delete(r.subscribers, id)
		close(subscriber)
	}
}

func (r *Registry) SubscriberCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.subscribers)
}

func (r *Registry) broadcastLocked(snapshot Snapshot) {
	cloned := cloneSnapshot(snapshot)
	for _, subscriber := range r.subscribers {
		select {
		case subscriber <- cloned:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- cloned:
			default:
			}
		}
	}
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	cloned := snapshot
	if snapshot.StartedAt != nil {
		startedAt := *snapshot.StartedAt
		cloned.StartedAt = &startedAt
	}
	if snapshot.FinishedAt != nil {
		finishedAt := *snapshot.FinishedAt
		cloned.FinishedAt = &finishedAt
	}
	cloned.Result = cloneResult(snapshot.Result)
	cloned.Error = cloneError(snapshot.Error)
	return cloned
}

func cloneResult(result *ResultSummary) *ResultSummary {
	if result == nil {
		return nil
	}

	cloned := &ResultSummary{
		Summary: result.Summary,
	}
	if result.Details != nil {
		cloned.Details = cloneMap(result.Details)
	}
	return cloned
}

func cloneError(errSummary *ErrorSummary) *ErrorSummary {
	if errSummary == nil {
		return nil
	}

	cloned := &ErrorSummary{
		Code:    errSummary.Code,
		Message: errSummary.Message,
	}
	if errSummary.Details != nil {
		cloned.Details = cloneMap(errSummary.Details)
	}
	return cloned
}

func cloneMap(source map[string]any) map[string]any {
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
