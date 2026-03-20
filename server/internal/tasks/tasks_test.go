package tasks

import (
	"testing"

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
