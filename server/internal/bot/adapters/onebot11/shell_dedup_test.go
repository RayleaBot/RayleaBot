package onebot11

import (
	"testing"
	"time"
)

func TestIsDuplicateEventHonoursRetentionWithoutSweeping(t *testing.T) {
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

func TestIsDuplicateEventSweepsWhenTheSetGrows(t *testing.T) {
	t.Parallel()

	shell := &Shell{recentEventIDs: make(map[string]time.Time)}
	base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	for i := 0; i <= recentEventDedupPruneSize; i++ {
		shell.isDuplicateEvent("old-"+time.Duration(i).String(), base)
	}
	shell.isDuplicateEvent("fresh", base.Add(recentEventDedupRetention+time.Second))
	if len(shell.recentEventIDs) != 1 {
		t.Fatalf("expected the expired ids to be swept, have %d entries", len(shell.recentEventIDs))
	}
}
