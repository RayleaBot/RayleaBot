package desktop

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLogsUseUTCDateAndTimestampAcrossSourceTimezones(t *testing.T) {
	directory := t.TempDir()
	before := time.Date(2026, 9, 22, 23, 59, 59, 123456789, time.UTC)
	appendLogAt(directory, "stderr", "first\n", before.In(time.FixedZone("east", 8*3600)))
	appendLogAt(directory, "stderr", "second\n", before.Add(2*time.Second).In(time.FixedZone("west", -7*3600)))
	for file, want := range map[string]string{
		"2026-09-22.log": "[2026-09-22T23:59:59.123456789Z] [stderr] first\n",
		"2026-09-23.log": "[2026-09-23T00:00:01.123456789Z] [stderr] second\n",
	} {
		got, err := os.ReadFile(filepath.Join(directory, file))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v", file, got, err)
		}
	}
}

func TestLogsPruneExpiredDatesAndPreserveOtherFiles(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"2026-09-29.log", "2026-09-30.log", "2026-10-01.log", "2026-10-07.log", "notes.log", "2026-99-01.log"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(directory, "2026-09-28.log"), 0o755); err != nil {
		t.Fatal(err)
	}
	appendLogAt(directory, "stdout", "today\n", time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC))
	if _, err := os.Stat(filepath.Join(directory, "2026-09-29.log")); !os.IsNotExist(err) {
		t.Fatalf("expired log remains: %v", err)
	}
	for _, name := range []string{"2026-09-30.log", "2026-10-01.log", "2026-10-06.log", "2026-10-07.log", "notes.log", "2026-99-01.log", "2026-09-28.log"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("retained entry %s: %v", name, err)
		}
	}
}
