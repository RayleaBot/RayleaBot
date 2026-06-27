package tasks

import (
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
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

func TestCreateWritesTaskLogForEveryFrozenTaskType(t *testing.T) {
	registry := NewRegistry()
	logs := logging.NewStream(16)
	registry.SetLogSink(logs)

	taskTypes := []string{
		"plugin.install",
		"plugin.uninstall",
		"plugin.reload",
		"backup.create",
		"recovery.recheck",
		"recovery.confirm",
		"restore.apply",
		"runtime.bootstrap",
	}

	for _, taskType := range taskTypes {
		if _, err := registry.Create(taskType, "summary for "+taskType); err != nil {
			t.Fatalf("Create(%q) returned unexpected error: %v", taskType, err)
		}
	}

	seen := map[string]bool{}
	for _, summary := range logs.Snapshot() {
		if summary.Source != "tasks" {
			continue
		}
		taskType, _ := summary.Details["task_type"].(string)
		if taskType == "" {
			t.Fatalf("task log missing task_type: %#v", summary.Details)
		}
		if summary.Details["task_status"] != string(StatusPending) {
			t.Fatalf("task log status for %s = %#v, want %q", taskType, summary.Details["task_status"], StatusPending)
		}
		if !strings.Contains(summary.Message, taskType) {
			t.Fatalf("task log message %q does not include task type %q", summary.Message, taskType)
		}
		seen[taskType] = true
	}

	for _, taskType := range taskTypes {
		if !seen[taskType] {
			t.Fatalf("missing task log for task type %q; logs=%#v", taskType, logs.Snapshot())
		}
	}
}

func TestUpdateWritesTaskLogForTerminalStatus(t *testing.T) {
	registry := NewRegistry()
	logs := logging.NewStream(8)
	registry.SetLogSink(logs)

	taskID, err := registry.Create("plugin.reload", "reload plugin")
	if err != nil {
		t.Fatalf("Create returned unexpected error: %v", err)
	}

	status := StatusFailed
	now := time.Date(2026, time.March, 20, 10, 0, 0, 0, time.UTC)
	if _, ok := registry.Update(taskID, Update{
		Status:     &status,
		Summary:    strPtrForTest("插件重载失败"),
		FinishedAt: &now,
		Error: &ErrorSummary{
			Code:    "plugin.internal_error",
			Message: "插件重载失败",
			Details: map[string]any{
				"plugin_id": "weather",
			},
		},
	}); !ok {
		t.Fatalf("Update(%q) returned ok=false", taskID)
	}

	summaries := logs.Snapshot()
	last := summaries[len(summaries)-1]
	if last.Level != "error" {
		t.Fatalf("terminal task log level = %q, want error", last.Level)
	}
	if last.PluginID != "weather" {
		t.Fatalf("terminal task log plugin_id = %q, want weather", last.PluginID)
	}
	if last.Details["task_status"] != string(StatusFailed) {
		t.Fatalf("terminal task log status = %#v, want %q", last.Details["task_status"], StatusFailed)
	}
	if last.Details["error_code"] != "plugin.internal_error" {
		t.Fatalf("terminal task log error_code = %#v", last.Details["error_code"])
	}
}

func strPtrForTest(value string) *string {
	return &value
}
