package tasks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/pubsub"
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

var (
	ErrQueueFull      = errors.New("task queue is full")
	ErrRegistryClosed = errors.New("task registry is closed")
)

type persistRequest struct {
	snapshot *Snapshot
	done     chan error
}

type Registry struct {
	mu    sync.RWMutex
	items map[string]Snapshot
	order []string
	hub   pubsub.Hub[Snapshot]
	repo  Repository
	logs  LogSink

	persistMu      sync.Mutex
	persistCond    *sync.Cond
	persistQueue   []persistRequest
	persistClosing bool
	persistStarted bool
	persistDone    chan struct{}
	persistErr     error
}

func NewRegistry() *Registry {
	registry := &Registry{
		items: map[string]Snapshot{},
		order: []string{},
	}
	registry.persistCond = sync.NewCond(&registry.persistMu)
	return registry
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

func (r *Registry) SetRepository(repo Repository) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.repo = repo
}

func (r *Registry) SetLogSink(logs LogSink) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = logs
}

func (r *Registry) Hydrate(ctx context.Context) error {
	r.mu.RLock()
	repo := r.repo
	r.mu.RUnlock()

	if repo == nil {
		return nil
	}
	if err := repo.InterruptInProgressTasks(ctx, time.Now().UTC()); err != nil {
		return fmt.Errorf("reconcile interrupted tasks: %w", err)
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
	r.mu.RLock()
	hasRepository := r.repo != nil
	r.mu.RUnlock()
	if hasRepository {
		if err := r.persist(snapshot, true); err != nil {
			return "", fmt.Errorf("persist new task: %w", err)
		}
	}

	r.mu.Lock()
	r.items[taskID] = snapshot
	r.order = append(r.order, taskID)
	r.broadcastLocked(snapshot)
	logs := r.logs
	r.mu.Unlock()

	appendTaskLog(logs, snapshot, taskLogEventCreated)

	return taskID, nil
}

func (r *Registry) Update(taskID string, update Update) (Snapshot, bool) {
	r.mu.Lock()

	snapshot, ok := r.items[taskID]
	if !ok {
		r.mu.Unlock()
		return Snapshot{}, false
	}

	previousStatus := snapshot.Status
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
	logs := r.logs
	if r.repo != nil {
		_ = r.persist(cloned, false)
	}
	r.mu.Unlock()

	if update.Status != nil && snapshot.Status != previousStatus {
		appendTaskLog(logs, cloned, taskLogEventStatusChanged)
	}

	return cloned, true
}

func (r *Registry) Subscribe(buffer int) (<-chan Snapshot, func()) {
	return r.hub.Subscribe(buffer)
}

func (r *Registry) SubscriberCount() int {
	return r.hub.SubscriberCount()
}

func (r *Registry) broadcastLocked(snapshot Snapshot) {
	r.hub.PublishReplace(cloneSnapshot(snapshot))
}

func (r *Registry) Flush(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	r.persistMu.Lock()
	if !r.persistStarted {
		err := r.persistErr
		r.persistMu.Unlock()
		return err
	}
	r.persistMu.Unlock()
	done := make(chan error, 1)
	if err := r.enqueuePersistence(persistRequest{done: done}); err != nil {
		return err
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *Registry) Close() error {
	if r == nil {
		return nil
	}
	r.persistMu.Lock()
	if !r.persistClosing {
		r.persistClosing = true
		r.persistCond.Broadcast()
	}
	if !r.persistStarted {
		err := r.persistErr
		r.persistMu.Unlock()
		return err
	}
	done := r.persistDone
	r.persistMu.Unlock()

	<-done
	r.persistMu.Lock()
	defer r.persistMu.Unlock()
	return r.persistErr
}

func (r *Registry) persist(snapshot Snapshot, wait bool) error {
	var done chan error
	if wait {
		done = make(chan error, 1)
	}
	cloned := cloneSnapshot(snapshot)
	if err := r.enqueuePersistence(persistRequest{snapshot: &cloned, done: done}); err != nil {
		return err
	}
	if done == nil {
		return nil
	}
	return <-done
}

func (r *Registry) enqueuePersistence(request persistRequest) error {
	r.persistMu.Lock()
	defer r.persistMu.Unlock()
	if r.persistClosing {
		return ErrRegistryClosed
	}
	if !r.persistStarted {
		r.persistStarted = true
		r.persistDone = make(chan struct{})
		go r.runPersistence()
	}
	r.persistQueue = append(r.persistQueue, request)
	r.persistCond.Signal()
	return nil
}

func (r *Registry) runPersistence() {
	defer close(r.persistDone)
	for {
		r.persistMu.Lock()
		for len(r.persistQueue) == 0 && !r.persistClosing {
			r.persistCond.Wait()
		}
		if len(r.persistQueue) == 0 && r.persistClosing {
			r.persistMu.Unlock()
			return
		}
		request := r.persistQueue[0]
		r.persistQueue[0] = persistRequest{}
		r.persistQueue = r.persistQueue[1:]
		r.persistMu.Unlock()

		err := r.savePersistedSnapshot(request.snapshot)
		if err != nil {
			r.persistMu.Lock()
			if r.persistErr == nil {
				r.persistErr = err
			}
			r.persistMu.Unlock()
		}
		if request.done != nil {
			request.done <- err
			close(request.done)
		}
	}
}

func (r *Registry) savePersistedSnapshot(snapshot *Snapshot) error {
	if snapshot == nil {
		r.persistMu.Lock()
		err := r.persistErr
		r.persistMu.Unlock()
		return err
	}

	r.mu.RLock()
	repo := r.repo
	r.mu.RUnlock()
	if repo == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return repo.SaveTask(ctx, *snapshot)
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

type taskLogEvent string

const (
	taskLogEventCreated       taskLogEvent = "created"
	taskLogEventStatusChanged taskLogEvent = "status_changed"
)

func appendTaskLog(logs LogSink, snapshot Snapshot, event taskLogEvent) {
	if logs == nil {
		return
	}

	details := taskLogDetails(snapshot, event)
	logs.Append(logging.Summary{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     taskLogLevel(snapshot.Status),
		Source:    "tasks",
		Message:   taskLogMessage(snapshot),
		PluginID:  readTaskDetailString(details, "plugin_id"),
		Protocol:  readTaskDetailString(details, "protocol"),
		RequestID: readTaskDetailString(details, "request_id"),
		Details:   details,
	})
}

func taskLogLevel(status Status) string {
	switch status {
	case StatusFailed:
		return "error"
	case StatusCancelled, StatusInterrupted:
		return "warn"
	default:
		return "info"
	}
}

func taskLogMessage(snapshot Snapshot) string {
	statusText := map[Status]string{
		StatusPending:     "任务已提交",
		StatusRunning:     "任务执行中",
		StatusSucceeded:   "任务完成",
		StatusFailed:      "任务失败",
		StatusCancelled:   "任务已取消",
		StatusInterrupted: "任务已中断",
	}[snapshot.Status]
	if statusText == "" {
		statusText = "任务状态更新"
	}

	summary := strings.TrimSpace(snapshot.Summary)
	if summary == "" {
		return fmt.Sprintf("%s %s", statusText, snapshot.TaskType)
	}
	return fmt.Sprintf("%s %s：%s", statusText, snapshot.TaskType, summary)
}

func taskLogDetails(snapshot Snapshot, event taskLogEvent) map[string]any {
	details := map[string]any{
		"task_event":   string(event),
		"task_id":      snapshot.TaskID,
		"task_type":    snapshot.TaskType,
		"task_status":  string(snapshot.Status),
		"task_summary": snapshot.Summary,
	}
	if snapshot.Progress > 0 {
		details["task_progress"] = snapshot.Progress
	}
	if snapshot.StartedAt != nil {
		details["started_at"] = snapshot.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if snapshot.FinishedAt != nil {
		details["finished_at"] = snapshot.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	if snapshot.Result != nil {
		details["result_summary"] = snapshot.Result.Summary
		if len(snapshot.Result.Details) > 0 {
			details["result_details"] = snapshot.Result.Details
		}
		mergeTaskContext(details, snapshot.Result.Details)
	}
	if snapshot.Error != nil {
		details["error_code"] = snapshot.Error.Code
		details["error_message"] = snapshot.Error.Message
		if len(snapshot.Error.Details) > 0 {
			details["error_details"] = snapshot.Error.Details
		}
		mergeTaskContext(details, snapshot.Error.Details)
	}
	return details
}

func mergeTaskContext(target map[string]any, source map[string]any) {
	for _, key := range []string{"plugin_id", "protocol", "request_id"} {
		if value := readTaskDetailString(source, key); value != "" {
			target[key] = value
		}
	}
}

func readTaskDetailString(source map[string]any, key string) string {
	if len(source) == 0 {
		return ""
	}
	value, _ := source[key].(string)
	return strings.TrimSpace(value)
}
