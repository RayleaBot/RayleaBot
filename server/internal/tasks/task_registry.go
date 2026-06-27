package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

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
	logs := r.logs
	r.mu.Unlock()

	r.persistAsync(repo, snapshot)
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
	repo := r.repo
	logs := r.logs
	r.mu.Unlock()

	r.persistAsync(repo, snapshot)
	if update.Status != nil && snapshot.Status != previousStatus {
		appendTaskLog(logs, cloned, taskLogEventStatusChanged)
	}

	return cloned, true
}
