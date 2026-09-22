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
