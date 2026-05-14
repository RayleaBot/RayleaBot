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
	defer executor.Close()

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
	defer executor.Close()

	taskID, err := executor.Submit("db.migrate", "test migration", func(ctx context.Context, p ProgressReporter) (*ResultSummary, error) {
		return nil, &TaskError{Code: "plugin.migration_failed", Message: "schema conflict"}
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		snap, _ := registry.Get(taskID)
		if snap.Status == StatusFailed {
			if snap.Error == nil || snap.Error.Code != "plugin.migration_failed" {
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
	defer executor.Close()

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
	defer executor.Close()

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


// recordingTaskMetrics captures every observed task execution outcome for
// assertions in TestExecutor_RecordsMetrics.
type recordingTaskMetrics struct {
	observations []taskMetricObservation
}

type taskMetricObservation struct {
	taskType string
	outcome  string
	duration time.Duration
}

func (m *recordingTaskMetrics) ObserveTaskExecution(taskType, outcome string, duration time.Duration) {
	m.observations = append(m.observations, taskMetricObservation{
		taskType: taskType,
		outcome:  outcome,
		duration: duration,
	})
}

// TestExecutor_RecordsMetrics verifies the executor calls the configured
// MetricsObserver for both successful and failed tasks. The observation
// for /api/system/metrics depends on this hook firing once per task.
func TestExecutor_RecordsMetrics(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	executor := NewExecutor(registry, 30*time.Second)
	defer executor.Close()

	metrics := &recordingTaskMetrics{}
	executor.SetMetricsObserver(metrics)

	successID, err := executor.Submit("backup.create", "ok", func(ctx context.Context, _ ProgressReporter) (*ResultSummary, error) {
		return &ResultSummary{Summary: "done"}, nil
	})
	if err != nil {
		t.Fatalf("submit success: %v", err)
	}
	failID, err := executor.Submit("backup.create", "boom", func(ctx context.Context, _ ProgressReporter) (*ResultSummary, error) {
		return nil, errors.New("boom")
	})
	if err != nil {
		t.Fatalf("submit fail: %v", err)
	}

	waitForFinalStatus(t, registry, successID, StatusSucceeded)
	waitForFinalStatus(t, registry, failID, StatusFailed)

	// Allow the executor goroutine to record metrics.
	time.Sleep(20 * time.Millisecond)

	if len(metrics.observations) != 2 {
		t.Fatalf("observations = %d, want 2", len(metrics.observations))
	}
	outcomes := map[string]bool{}
	for _, obs := range metrics.observations {
		if obs.taskType != "backup.create" {
			t.Fatalf("taskType = %q, want backup.create", obs.taskType)
		}
		outcomes[obs.outcome] = true
	}
	if !outcomes["succeeded"] || !outcomes["failed"] {
		t.Fatalf("outcomes = %v, want both succeeded and failed", outcomes)
	}
}

func waitForFinalStatus(t *testing.T, registry *Registry, taskID string, want Status) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		snap, ok := registry.Get(taskID)
		if !ok {
			t.Fatalf("task %s not found", taskID)
		}
		if snap.Status == want {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for status %s on %s, last status=%s", want, taskID, snap.Status)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
