package dispatch

import "testing"

func TestStatsTotalsPreservesCountersAndFullSnapshotOwnership(t *testing.T) {
	d := New(nil, nil, nil, 1)
	t.Cleanup(d.Close)
	for _, outcome := range []Outcome{OutcomeDelivered, OutcomeDropped, OutcomeError, OutcomeIgnored} {
		d.recordOutcome(outcome, "fixture", "queue_full")
	}
	want := DispatcherTotals{Delivered: 1, Dropped: 1, Errored: 1, Ignored: 1}
	if got := d.StatsTotals(); got != want {
		t.Fatalf("totals = %+v, want %+v", got, want)
	}
	full := d.Stats()
	full.DropsByReason["queue_full"]["fixture"] = 100
	delete(full.DropsByReason, "queue_full")
	if got := d.Stats(); got.DropsByReason["queue_full"]["fixture"] != 1 {
		t.Fatalf("full snapshot changed stored drop details: %+v", got)
	}
	d.recordOutcome(OutcomeDropped, "another", "queue_full")
	want.Dropped++
	if got := d.StatsTotals(); got != want {
		t.Fatalf("totals missed a subsequent outcome: %+v", got)
	}
}
