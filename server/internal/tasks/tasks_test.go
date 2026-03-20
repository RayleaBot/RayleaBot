package tasks

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Feature: plugin-write-api, Property 6: 任务 ID 唯一性
// Validates: Requirements 6.4
func TestProperty_CreateTaskIDUniqueness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(2, 50).Draw(t, "n")
		registry := NewRegistry()

		seen := make(map[string]struct{}, n)
		for i := 0; i < n; i++ {
			taskID, err := registry.Create("plugin.install", "test task")
			if err != nil {
				t.Fatalf("Create failed on iteration %d: %v", i, err)
			}
			if _, exists := seen[taskID]; exists {
				t.Fatalf("duplicate task_id %q on iteration %d of %d", taskID, i, n)
			}
			seen[taskID] = struct{}{}
		}
	})
}

// --- Unit tests for Registry.Create ---
// Validates: Requirements 6.1, 6.2, 6.3, 6.5

func TestCreate_ReturnsTaskIDWithCorrectFormat(t *testing.T) {
	registry := NewRegistry()
	taskID, err := registry.Create("plugin.install", "install from local zip")
	if err != nil {
		t.Fatalf("Create returned unexpected error: %v", err)
	}

	// task_id must have "task_" prefix followed by exactly 32 hex characters
	const prefix = "task_"
	if len(taskID) < len(prefix) || taskID[:len(prefix)] != prefix {
		t.Fatalf("task_id %q does not start with %q", taskID, prefix)
	}
	hexPart := taskID[len(prefix):]
	if len(hexPart) != 32 {
		t.Fatalf("hex part of task_id has length %d, want 32", len(hexPart))
	}
	for _, c := range hexPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("task_id hex part contains non-hex character %q", c)
		}
	}
}

func TestCreate_GetReturnsCorrectTaskTypeAndStatus(t *testing.T) {
	registry := NewRegistry()
	taskID, err := registry.Create("plugin.install", "install test plugin")
	if err != nil {
		t.Fatalf("Create returned unexpected error: %v", err)
	}

	snap, ok := registry.Get(taskID)
	if !ok {
		t.Fatalf("Get(%q) returned false, expected task to exist", taskID)
	}
	if snap.TaskType != "plugin.install" {
		t.Errorf("TaskType = %q, want %q", snap.TaskType, "plugin.install")
	}
	if snap.Status != StatusPending {
		t.Errorf("Status = %q, want %q", snap.Status, StatusPending)
	}
}

func TestCreate_ListIncludesNewTask(t *testing.T) {
	registry := NewRegistry()
	taskID, err := registry.Create("plugin.install", "install hello plugin")
	if err != nil {
		t.Fatalf("Create returned unexpected error: %v", err)
	}

	tasks := registry.List()
	found := false
	for _, snap := range tasks {
		if snap.TaskID == taskID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("List() does not contain newly created task %q", taskID)
	}
}

func TestUpdate_ReplacesTaskSnapshotAndPublishes(t *testing.T) {
	registry := NewRegistry()
	taskID, err := registry.Create("plugin.install", "install hello plugin")
	if err != nil {
		t.Fatalf("Create returned unexpected error: %v", err)
	}

	updates, unsubscribe := registry.Subscribe(2)
	defer unsubscribe()

	status := StatusRunning
	progress := 40
	summary := "安装 Python 依赖"
	startedAt := time.Date(2026, time.March, 20, 10, 0, 0, 0, time.UTC)
	result := &ResultSummary{Summary: "still running"}

	snapshot, ok := registry.Update(taskID, Update{
		Status:    &status,
		Progress:  &progress,
		Summary:   &summary,
		StartedAt: &startedAt,
		Result:    result,
	})
	if !ok {
		t.Fatalf("Update(%q) returned ok=false", taskID)
	}

	if snapshot.Status != StatusRunning {
		t.Fatalf("Status = %q, want %q", snapshot.Status, StatusRunning)
	}
	if snapshot.Progress != 40 {
		t.Fatalf("Progress = %d, want 40", snapshot.Progress)
	}
	if snapshot.Summary != "安装 Python 依赖" {
		t.Fatalf("Summary = %q, want %q", snapshot.Summary, "安装 Python 依赖")
	}
	if snapshot.StartedAt == nil || !snapshot.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %v, want %v", snapshot.StartedAt, startedAt)
	}

	select {
	case published := <-updates:
		if published.TaskID != taskID {
			t.Fatalf("published TaskID = %q, want %q", published.TaskID, taskID)
		}
		if published.Status != StatusRunning {
			t.Fatalf("published Status = %q, want %q", published.Status, StatusRunning)
		}
	default:
		t.Fatal("expected update to be published")
	}
}
