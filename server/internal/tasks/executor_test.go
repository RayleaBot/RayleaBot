package tasks

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestExecutor_SubmitAndSucceed(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	executor := NewExecutor(registry, 30*time.Second)
	defer func(release func() error) { _ = release() }(executor.Close)

	taskID, err := executor.Submit("backup.create", "test backup", func(ctx context.Context, p ProgressReporter) (*ResultSummary, error) {
		p.Update(50, "halfway")
		return &ResultSummary{Summary: "backup completed"}, nil
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if taskID == "" {
		t.Fatal("task_id is empty")
	}

	// Wait for completion.
	deadline := time.After(5 * time.Second)
	for {
		snap, ok := registry.Get(taskID)
		if !ok {
			t.Fatal("task not found")
		}
		if snap.Status == StatusSucceeded {
			if snap.Result == nil || snap.Result.Summary != "backup completed" {
				t.Fatalf("unexpected result: %+v", snap.Result)
			}
			if snap.Progress != 100 {
				t.Fatalf("progress = %d, want 100", snap.Progress)
			}
			break
		}
		if snap.Status == StatusFailed {
			t.Fatalf("task failed: %+v", snap.Error)
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for task to succeed, status=%s", snap.Status)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestExecutor_SubmitAndFail(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	executor := NewExecutor(registry, 30*time.Second)
	defer func(release func() error) { _ = release() }(executor.Close)

	taskID, err := executor.Submit("plugin.reload", "test reload", func(ctx context.Context, p ProgressReporter) (*ResultSummary, error) {
		return nil, &TaskError{Code: "plugin.internal_error", Message: "runtime conflict"}
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		snap, _ := registry.Get(taskID)
		if snap.Status == StatusFailed {
			if snap.Error == nil || snap.Error.Code != "plugin.internal_error" {
				t.Fatalf("unexpected error: %+v", snap.Error)
			}
			break
		}
		if snap.Status == StatusSucceeded {
			t.Fatal("expected failure")
		}
		select {
		case <-deadline:
			t.Fatalf("timeout, status=%s", snap.Status)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestExecutor_SubmitGenericError(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	executor := NewExecutor(registry, 30*time.Second)
	defer func(release func() error) { _ = release() }(executor.Close)

	taskID, err := executor.Submit("restore.apply", "test restore", func(ctx context.Context, p ProgressReporter) (*ResultSummary, error) {
		return nil, errors.New("disk full")
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		snap, _ := registry.Get(taskID)
		if snap.Status == StatusFailed {
			if snap.Error == nil || snap.Error.Code != "platform.internal_error" {
				t.Fatalf("unexpected error code: %+v", snap.Error)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout, status=%s", snap.Status)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestExecutor_Cancel(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	executor := NewExecutor(registry, 30*time.Second)
	defer func(release func() error) { _ = release() }(executor.Close)

	started := make(chan struct{})
	blocked := make(chan struct{})

	// Submit a blocking task first to hold the executor.
	_, _ = executor.Submit("backup.create", "blocker", func(ctx context.Context, p ProgressReporter) (*ResultSummary, error) {
		close(started)
		<-blocked
		return &ResultSummary{Summary: "done"}, nil
	})

	<-started

	// Submit a second task that will be pending.
	taskID, err := executor.Submit("backup.create", "to cancel", func(ctx context.Context, p ProgressReporter) (*ResultSummary, error) {
		return &ResultSummary{Summary: "should not run"}, nil
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	ok := executor.Cancel(taskID)
	if !ok {
		t.Fatal("cancel returned false")
	}

	snap, _ := registry.Get(taskID)
	if snap.Status != StatusCancelled {
		t.Fatalf("status = %s, want cancelled", snap.Status)
	}

	close(blocked)
}

func TestExecutor_Close(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	executor := NewExecutor(registry, 30*time.Second)

	if err := executor.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Submit after close should fail.
	_, err := executor.Submit("backup.create", "after close", func(ctx context.Context, p ProgressReporter) (*ResultSummary, error) {
		return nil, nil
	})
	if err == nil {
		t.Fatal("expected error after close")
	}
}

func TestExecutorRejectsFullQueueBeforeCreatingTask(t *testing.T) {
	registry := NewRegistry()
	executor := NewExecutor(registry, 30*time.Second)

	started := make(chan struct{})
	release := make(chan struct{})
	if _, err := executor.Submit("backup.create", "running", func(context.Context, ProgressReporter) (*ResultSummary, error) {
		close(started)
		<-release
		return &ResultSummary{Summary: "done"}, nil
	}); err != nil {
		t.Fatalf("submit running task: %v", err)
	}
	<-started

	for index := 0; index < 32; index++ {
		if _, err := executor.Submit("backup.create", "queued", func(context.Context, ProgressReporter) (*ResultSummary, error) {
			return &ResultSummary{Summary: "done"}, nil
		}); err != nil {
			t.Fatalf("submit queued task %d: %v", index, err)
		}
	}
	before := len(registry.List())
	if _, err := executor.Submit("backup.create", "rejected", func(context.Context, ProgressReporter) (*ResultSummary, error) {
		return nil, nil
	}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("queue-full error = %v, want ErrQueueFull", err)
	}
	if after := len(registry.List()); after != before {
		t.Fatalf("queue-full submission created a task: before=%d after=%d", before, after)
	}

	close(release)
	if err := executor.Close(); err != nil {
		t.Fatalf("close executor: %v", err)
	}
}
