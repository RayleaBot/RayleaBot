package messagestats

import (
	"fmt"
	"testing"
	"time"
)

func TestOfflineGraceThresholdAndLifecycle(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	incidents := func() []Incident {
		return queryStats(t, s, "2026-10-01T00:00:00Z", "2026-10-02T00:00:00Z", "hour").Incidents
	}
	observe := func(enabled, connected bool) {
		s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: enabled, Connected: connected}})
	}
	clock.add(89 * time.Second)
	if got := incidents(); len(got) != 0 {
		t.Fatalf("reported before grace+threshold: %+v", got)
	}
	clock.add(time.Second)
	got := incidents()
	if len(got) != 1 || got[0].StartedAt.Format(time.RFC3339) != "2026-10-01T00:01:00Z" || got[0].EndedAt != nil {
		t.Fatalf("startup interval=%+v", got)
	}
	if err := s.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	observe(true, true)
	got = incidents()
	if got[0].EndedAt == nil || !got[0].EndedAt.Equal(clock.now()) {
		t.Fatalf("connection did not close interval: %+v", got)
	}
	clock.add(time.Second)
	observe(true, false)
	clock.add(29 * time.Second)
	observe(true, true)
	if got := incidents(); len(got) != 1 {
		t.Fatalf("short disconnect reported: %+v", got)
	}
	observe(true, false)
	start := clock.now()
	clock.add(30 * time.Second)
	observe(false, false)
	got = incidents()
	if len(got) != 2 || !got[1].StartedAt.Equal(start) || got[1].EndedAt == nil {
		t.Fatalf("disable did not close: %+v", got)
	}
	observe(true, false)
	clock.add(89 * time.Second)
	if len(incidents()) != 2 {
		t.Fatal("enable grace missing")
	}
	clock.add(time.Second)
	s.ObserveAdapters(nil)
	got = incidents()
	if len(got) != 3 || got[2].EndedAt == nil {
		t.Fatalf("removal did not close: %+v", got)
	}
	observe(true, true)
	observe(true, false)
	clock.add(30 * time.Second)
	if err := s.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	got = incidents()
	if len(got) != 4 || got[3].EndedAt == nil {
		t.Fatalf("stop did not close: %+v", got)
	}
}

func TestOfflineReloadAndCrashRecovery(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true, Connected: true}})
	clock.add(time.Second)
	s.ReloadAdapter("bot")
	s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true}})
	clock.add(89 * time.Second)
	view := queryStats(t, s, "2026-10-01T00:00:00Z", "2026-10-02T00:00:00Z", "hour")
	if len(view.Incidents) != 0 {
		t.Fatal("reload skipped grace")
	}
	clock.add(time.Second)
	if err := s.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	lastAlive := clock.now()
	s.loopCancel()
	<-s.loopDone
	clock.add(5 * time.Minute)
	resumed, err := New(t.Context(), Options{Store: s.store, Now: clock.now, Timezone: "UTC", CurrentConfig: s.config})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resumed.loopCancel(); <-resumed.loopDone })
	view = queryStats(t, resumed, "2026-10-01T00:00:00Z", "2026-10-01T00:01:00Z", "hour")
	// Incidents intentionally intersect [start_at, as_of), even when end_at is earlier.
	if len(view.Incidents) != 2 || view.Incidents[0].EndedAt == nil || !view.Incidents[0].EndedAt.Equal(lastAlive) || view.Incidents[1].Kind != "server_stopped" || view.Incidents[1].AdapterID != "" || !view.Incidents[1].StartedAt.Equal(lastAlive) || !view.Incidents[1].EndedAt.Equal(clock.now()) {
		t.Fatalf("crash history=%+v", view.Incidents)
	}
	clock.add(10 * time.Second)
	if err := resumed.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	gracefulStop := clock.now()
	clock.add(2 * time.Minute)
	next, err := New(t.Context(), Options{Store: s.store, Now: clock.now, Timezone: "UTC", CurrentConfig: s.config})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { next.loopCancel(); <-next.loopDone })
	view = queryStats(t, next, "2026-10-01T00:03:00Z", "2026-10-01T00:04:00Z", "hour")
	if len(view.Incidents) != 3 || !view.Incidents[2].StartedAt.Equal(gracefulStop) {
		t.Fatalf("graceful gap=%+v", view.Incidents)
	}
}

func TestLatestThousandIncidentsAcrossFlushedAndPending(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	for i := range 1005 {
		id := fmt.Sprintf("bot-%04d", i)
		s.ObserveAdapters([]Adapter{{ID: id, Enabled: true, Connected: true}})
		s.ObserveAdapters([]Adapter{{ID: id, Enabled: true}})
		clock.add(30 * time.Second)
		s.ObserveAdapters(nil)
		if i%200 == 0 {
			if err := s.Flush(t.Context()); err != nil {
				t.Fatal(err)
			}
		}
	}
	view := queryStats(t, s, "2026-10-01T00:00:00Z", "2026-10-02T00:00:00Z", "hour")
	if !view.IncidentsTruncated || len(view.Incidents) != 1000 || view.Incidents[0].AdapterID != "bot-0005" || view.Incidents[999].AdapterID != "bot-1004" {
		t.Fatalf("bad incident truncation: %d, %v", len(view.Incidents), view.IncidentsTruncated)
	}
	for i := 1; i < len(view.Incidents); i++ {
		if view.Incidents[i].StartedAt.Before(view.Incidents[i-1].StartedAt) {
			t.Fatal("incidents out of order")
		}
	}
}

func TestClosedPendingIntervalDoesNotReappearFromPersistedOpenRow(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	clock.add(90 * time.Second)
	if err := s.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.ObserveAdapters(nil)
	clock.add(90 * time.Minute)
	view := queryStats(t, s, "2026-10-01T01:00:00Z", "2026-10-01T02:00:00Z", "hour")
	if len(view.Incidents) != 0 {
		t.Fatalf("closed incident reappeared: %+v", view.Incidents)
	}
}
