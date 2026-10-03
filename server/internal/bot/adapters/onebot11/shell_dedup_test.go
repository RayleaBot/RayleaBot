package onebot11

import (
	"testing"
	"time"
)

func TestIsDuplicateEventHonoursRetention(t *testing.T) {
	t.Parallel()

	shell := &Shell{recentEventIDs: make(map[string]time.Time)}
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	if shell.isDuplicateEvent("evt-1", base) {
		t.Fatal("first observation must not be a duplicate")
	}
	if !shell.isDuplicateEvent("evt-1", base.Add(time.Second)) {
		t.Fatal("second observation inside the window must be a duplicate")
	}
	if shell.isDuplicateEvent("evt-1", base.Add(recentEventDedupRetention+time.Second)) {
		t.Fatal("an id older than the retention window must be accepted again")
	}
	if got := shell.DedupDropsSnapshot(); got != 1 {
		t.Fatalf("dedup drops = %d, want 1", got)
	}
	if shell.isDuplicateEvent("", base) {
		t.Fatal("blank ids are never deduplicated")
	}
}

func TestIsDuplicateEventExpiresAnIdleWindow(t *testing.T) {
	t.Parallel()

	shell := &Shell{recentEventIDs: make(map[string]time.Time)}
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 2048; i++ {
		shell.isDuplicateEvent("old-"+time.Duration(i).String(), base)
	}
	shell.isDuplicateEvent("fresh", base.Add(recentEventDedupRetention+time.Second))
	if len(shell.recentEventIDs) != 1 {
		t.Fatalf("expected the expired ids to be swept, have %d entries", len(shell.recentEventIDs))
	}
}

func TestDedupExpiresOutOfOrderObservationsAndPreservesCutoff(t *testing.T) {
	shell := &Shell{recentEventIDs: make(map[string]time.Time)}
	base := time.Unix(1700000000, 0)
	for _, item := range []struct {
		id     string
		offset time.Duration
	}{{"later", time.Second}, {"earlier", 0}, {"latest", 2 * time.Second}} {
		if shell.isDuplicateEvent(item.id, base.Add(item.offset)) {
			t.Fatal("new event rejected")
		}
	}
	if !shell.isDuplicateEvent("earlier", base.Add(recentEventDedupRetention)) {
		t.Fatal("cutoff must remain included")
	}
	if shell.isDuplicateEvent("earlier", base.Add(recentEventDedupRetention+time.Nanosecond)) {
		t.Fatal("expired observation rejected")
	}
	if !shell.isDuplicateEvent("later", base.Add(recentEventDedupRetention+time.Second)) {
		t.Fatal("unexpired observation was removed")
	}
	shell.resetDedup()
	if shell.isDuplicateEvent("later", base.Add(recentEventDedupRetention+time.Second)) {
		t.Fatal("reset retained dedup state")
	}
	if shell.DedupDropsSnapshot() != 2 {
		t.Fatal("reset changed cumulative drop count")
	}
}

func TestDedupReleasesExpiredBurstWhileRecentTrafficContinues(t *testing.T) {
	shell := &Shell{recentEventIDs: make(map[string]time.Time)}
	base := time.Unix(1700000000, 0)
	for i := range 4096 {
		shell.isDuplicateEvent(time.Duration(i).String(), base)
	}
	shell.isDuplicateEvent("still-live", base.Add(time.Minute))
	shell.isDuplicateEvent("new", base.Add(recentEventDedupRetention+time.Second))
	if len(shell.recentEventIDs) != 2 || cap(shell.recentEvents) > 1024 {
		t.Fatal("expired burst retained its records or storage")
	}
	if !shell.isDuplicateEvent("still-live", base.Add(recentEventDedupRetention+time.Second)) {
		t.Fatal("live event was lost during shrink")
	}
}

func TestDedupRetainsLateObservationsUntilTheExistingPruneBoundary(t *testing.T) {
	shell := &Shell{recentEventIDs: make(map[string]time.Time)}
	base := time.Unix(1700000000, 0)
	shell.isDuplicateEvent("first", base)
	shell.isDuplicateEvent("prune", base.Add(115*time.Second))
	shell.isDuplicateEvent("newer", base.Add(130*time.Second))
	if !shell.isDuplicateEvent("first", base.Add(119*time.Second)) {
		t.Fatal("late observation inside the retained window was admitted twice")
	}
	if shell.isDuplicateEvent("first", base.Add(131*time.Second)) {
		t.Fatal("expired observation did not refresh")
	}
	shell.isDuplicateEvent("next-prune", base.Add(146*time.Second))
	if !shell.isDuplicateEvent("first", base.Add(147*time.Second)) {
		t.Fatal("old expiry removed a refreshed observation")
	}
}
