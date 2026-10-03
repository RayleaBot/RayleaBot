package tasks

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func awaitTerminalTask(t *testing.T, registry *Registry, id string) Snapshot {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		snapshot, _ := registry.Get(id)
		if snapshot.Status != StatusPending && snapshot.Status != StatusRunning {
			return snapshot
		}
		select {
		case <-deadline:
			t.Fatalf("task remained %s", snapshot.Status)
		case <-time.After(time.Millisecond):
		}
	}
}

func TestExecutorCancellationOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status Status
		code   string
	}{
		{"caller", StatusCancelled, ""},
		{"shutdown", StatusInterrupted, ""},
		{"timeout", StatusFailed, "platform.task_timeout"},
		{"real failure during shutdown", StatusFailed, "plugin.internal_error"},
		{"cleanup failure during shutdown", StatusFailed, "platform.internal_error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := NewRegistry()
			timeout := time.Minute
			if tc.name == "timeout" {
				timeout = 20 * time.Millisecond
			}
			executor := NewExecutor(registry, timeout)
			t.Cleanup(func() { _ = executor.Close() })
			started := make(chan struct{})
			id, err := executor.Submit("fixture", "fixture", func(ctx context.Context, _ ProgressReporter) (*ResultSummary, error) {
				close(started)
				if tc.name == "caller" {
					return nil, fmt.Errorf("caller ended: %w", context.Canceled)
				}
				<-ctx.Done()
				if tc.name == "real failure during shutdown" {
					return nil, errors.Join(ctx.Err(), fmt.Errorf("rollback: %w", &TaskError{Code: "plugin.internal_error", Message: "rollback failed"}))
				}
				if tc.name == "cleanup failure during shutdown" {
					return nil, fmt.Errorf("cleanup: %w", errors.Join(ctx.Err(), errors.New("cleanup failed")))
				}
				return nil, fmt.Errorf("work ended: %w", ctx.Err())
			})
			if err != nil {
				t.Fatal(err)
			}
			<-started
			switch tc.name {
			case "shutdown", "real failure during shutdown", "cleanup failure during shutdown":
				if err := executor.Close(); err != nil {
					t.Fatal(err)
				}
			}
			snapshot := awaitTerminalTask(t, registry, id)
			if snapshot.Status != tc.status || snapshot.FinishedAt == nil {
				t.Fatalf("snapshot = %#v", snapshot)
			}
			if tc.code == "" {
				if snapshot.Error != nil {
					t.Fatalf("cancellation recorded an error: %#v", snapshot.Error)
				}
			} else if snapshot.Error == nil || snapshot.Error.Code != tc.code {
				t.Fatalf("error = %#v", snapshot.Error)
			}
		})
	}
}

func TestExecutorShutdownSettlesQueuedTasks(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry, time.Minute)
	t.Cleanup(func() { _ = executor.Close() })
	started, release := make(chan struct{}), make(chan struct{})
	running, err := executor.Submit("fixture", "fixture", func(ctx context.Context, _ ProgressReporter) (*ResultSummary, error) {
		close(started)
		<-ctx.Done()
		<-release
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started
	queued, err := executor.Submit("fixture", "fixture", func(context.Context, ProgressReporter) (*ResultSummary, error) {
		return nil, errors.New("queued work must not execute")
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- executor.Close() }()
	<-executor.baseCtx.Done()
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := awaitTerminalTask(t, registry, running); got.Status != StatusInterrupted {
		t.Fatal(got)
	}
	if got := awaitTerminalTask(t, registry, queued); got.Status != StatusInterrupted {
		t.Fatal(got)
	}
}
