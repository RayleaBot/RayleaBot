package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestTaskStatusesMatchFrozenContractValues(t *testing.T) {
	t.Parallel()

	got := []string{
		string(tasks.StatusPending),
		string(tasks.StatusRunning),
		string(tasks.StatusSucceeded),
		string(tasks.StatusFailed),
		string(tasks.StatusCancelled),
		string(tasks.StatusInterrupted),
	}
	want := []string{
		"pending",
		"running",
		"succeeded",
		"failed",
		"cancelled",
		"interrupted",
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected status count: got %d want %d", len(got), len(want))
	}

	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("unexpected status at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestEmptyRegistryIsReadonlyAndStable(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	if list := registry.List(); len(list) != 0 {
		t.Fatalf("expected empty list, got %d item(s)", len(list))
	}

	if _, ok := registry.Get("missing"); ok {
		t.Fatal("expected missing task lookup to return false")
	}
}

func TestTaskSnapshotJSONRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 17, 8, 30, 0, 0, time.UTC)
	snapshot := tasks.Snapshot{
		TaskID:    "task_hello_0001",
		TaskType:  "plugin.install",
		Status:    tasks.StatusRunning,
		Progress:  50,
		Summary:   "installing hello plugin",
		StartedAt: &now,
		Result: &tasks.ResultSummary{
			Summary: "halfway done",
		},
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	if body["status"] != "running" {
		t.Fatalf("unexpected status encoding: %#v", body["status"])
	}
	if body["task_id"] != "task_hello_0001" {
		t.Fatalf("unexpected task_id encoding: %#v", body["task_id"])
	}
}
