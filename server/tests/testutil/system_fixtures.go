package testutil

import (
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

// WritePreparedRuntime marks a Chromium entrypoint as prepared in the repo-local .deps store.
func WritePreparedRuntime(t testing.TB, repoRoot, id, version string, segments ...string) {
	t.Helper()
	WriteTestRuntimeEntry(t, repoRoot, id, version, segments...)
}

// WaitTask polls the registry until the task reaches the wanted status or the
// deadline passes.
func WaitTask(t testing.TB, registry *tasks.Registry, taskID string, want tasks.Status) tasks.Snapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snapshot, ok := registry.Get(taskID)
		if ok && snapshot.Status == want {
			return snapshot
		}
		time.Sleep(20 * time.Millisecond)
	}
	snapshot, _ := registry.Get(taskID)
	t.Fatalf("task %s did not reach %s: %#v", taskID, want, snapshot)
	return tasks.Snapshot{}
}
