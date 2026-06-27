package tasks

import (
	"context"
	"fmt"
	"time"
)

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
