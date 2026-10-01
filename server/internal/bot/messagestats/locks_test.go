package messagestats

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"
)

func statsUpdateWithin(t *testing.T, update func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		update()
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("memory update blocked on database I/O")
	}
}

func waitForStatsDBWait(t *testing.T, db *sql.DB, previous int64) {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for db.Stats().WaitCount == previous {
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatal("statistics operation did not reach the occupied database connection")
		}
	}
}

func TestMemoryUpdatesDoNotWaitForIO(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	clock.add(90 * time.Second)
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	for _, tc := range []struct {
		name   string
		update func()
	}{
		{"received", func() { s.count("bot", "onebot11", true) }},
		{"sent", func() { s.count("bot", "onebot11", false) }},
		{"observe", func() { s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true, Connected: true}}) }},
		{"reload", func() { s.ReloadAdapter("bot") }},
	} {
		t.Run(tc.name, func(t *testing.T) { statsUpdateWithin(t, tc.update) })
	}
}

func TestFailedFlushMergesConcurrentCountsAndMetadata(t *testing.T) {
	for _, tc := range []struct {
		name       string
		advance    time.Duration
		received   bool
		wantCounts Counts
	}{
		{"later_receive", time.Second, true, Counts{2, 1}},
		{"earlier_receive", -time.Second, true, Counts{2, 1}},
		{"send_only", time.Second, false, Counts{1, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
			clock.add(90 * time.Second)
			receive(s, "bot")
			receive(s, "old-only")
			latest := clock.now()
			endedAt := clock.now()
			s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true, Connected: true}})
			// Fail after counts, metadata and intervals have been written in the transaction.
			if _, err := s.store.Write.ExecContext(t.Context(), `CREATE TRIGGER fail_stats_flush
				BEFORE UPDATE ON message_stats_runs BEGIN SELECT RAISE(ABORT, 'test flush failure'); END`); err != nil {
				t.Fatal(err)
			}
			conn, err := s.store.Write.Conn(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			waits := s.store.Write.Stats().WaitCount
			flushed := make(chan error, 1)
			go func() { flushed <- s.Flush(t.Context()) }()
			waitForStatsDBWait(t, s.store.Write, waits)
			clock.add(tc.advance)
			statsUpdateWithin(t, func() {
				if tc.received {
					s.count("bot", "qqofficial", true)
				}
				s.Sent("bot", "qqofficial")
				receive(s, "new-only")
			})
			if tc.received && clock.now().After(latest) {
				latest = clock.now()
			}
			if err := conn.Close(); err != nil {
				t.Fatal(err)
			}
			if err := <-flushed; err == nil {
				t.Fatal("flush unexpectedly succeeded")
			}
			var persisted int
			if err := s.store.Read.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM message_stats_hours").Scan(&persisted); err != nil || persisted != 0 {
				t.Fatalf("failed transaction retained counts: %d, %v", persisted, err)
			}
			before := queryStats(t, s, "2026-10-01T00:00:00Z", "2026-10-01T01:00:00Z", "hour")
			if before.Connections[0].Totals != tc.wantCounts || before.Connections[0].LastReceivedAt == nil || !before.Connections[0].LastReceivedAt.Equal(latest) || before.Totals.Received != tc.wantCounts.Received+2 {
				t.Fatalf("failed flush lost concurrent counts or metadata: %+v", before)
			}
			if len(before.Incidents) != 1 || before.Incidents[0].EndedAt == nil || !before.Incidents[0].EndedAt.Equal(endedAt) {
				t.Fatalf("failed flush lost ended interval: %+v", before.Incidents)
			}
			if _, err := s.store.Write.ExecContext(t.Context(), "DROP TRIGGER fail_stats_flush"); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := s.Flush(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			after := queryStats(t, s, "2026-10-01T00:00:00Z", "2026-10-01T01:00:00Z", "hour")
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("retry changed response: before=%+v after=%+v", before, after)
			}
			var protocol string
			var last int64
			if err := s.store.Read.QueryRowContext(t.Context(), "SELECT protocol,last_received_at_ms FROM message_stats_adapters WHERE adapter_id='bot'").Scan(&protocol, &last); err != nil || protocol != "qqofficial" || last != latest.UnixMilli() {
				t.Fatalf("retry lost newer metadata: %s/%d, %v", protocol, last, err)
			}
		})
	}
}

func TestFlushPreservesIntervalsClosedDuringIO(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "commit"
		if fail {
			name = "cancel"
		}
		t.Run(name, func(t *testing.T) {
			s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
			clock.add(90 * time.Second)
			receive(s, "bot")
			conn, err := s.store.Write.Conn(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			waits := s.store.Write.Stats().WaitCount
			flushed := make(chan error, 1)
			go func() { flushed <- s.Flush(ctx) }()
			waitForStatsDBWait(t, s.store.Write, waits)
			clock.add(time.Second)
			statsUpdateWithin(t, func() {
				receive(s, "bot")
				s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true, Connected: true}})
			})
			queried := make(chan Response, 1)
			queryErr := make(chan error, 1)
			go func() {
				view, err := s.Query(t.Context(), Query{"2026-10-01T00:00:00Z", "2026-10-01T01:00:00Z", "hour"})
				queryErr <- err
				queried <- view
			}()
			select {
			case <-queried:
				t.Fatal("query returned while a flush batch was detached")
			case <-time.After(50 * time.Millisecond):
			}
			if fail {
				cancel()
			} else if err := conn.Close(); err != nil {
				t.Fatal(err)
			}
			if err := <-flushed; (fail && !errors.Is(err, context.Canceled)) || (!fail && err != nil) {
				t.Fatalf("flush error = %v", err)
			}
			if fail {
				if err := conn.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := <-queryErr; err != nil {
				t.Fatal(err)
			}
			if view := <-queried; view.Totals != (Counts{Received: 2}) {
				t.Fatalf("query lost or duplicated detached counts: %+v", view.Totals)
			}
			for range 2 {
				view := queryStats(t, s, "2026-10-01T00:00:00Z", "2026-10-01T01:00:00Z", "hour")
				if len(view.Incidents) != 1 || view.Incidents[0].EndedAt == nil || !view.Incidents[0].EndedAt.Equal(clock.now()) {
					t.Fatalf("flush lost or reopened closed interval: %+v", view.Incidents)
				}
				if err := s.Flush(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestStopRetriesFinalBatchAtOriginalStopTime(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	clock.add(90 * time.Second)
	receive(s, "bot")
	stoppedAt := clock.now()
	conn, err := s.store.Write.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	waits := s.store.Write.Stats().WaitCount
	stopped := make(chan error, 1)
	go func() { stopped <- s.Stop(ctx) }()
	waitForStatsDBWait(t, s.store.Write, waits)
	clock.add(time.Minute)
	statsUpdateWithin(t, func() {
		receive(s, "bot")
		s.Sent("bot", "onebot11")
		s.ObserveAdapters(nil)
		s.ReloadAdapter("bot")
	})
	cancel()
	if err := <-stopped; !errors.Is(err, context.Canceled) {
		t.Fatalf("final flush ignored cancellation: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	view := queryStats(t, s, "2026-10-01T00:00:00Z", "2026-10-01T01:00:00Z", "hour")
	if view.Totals != (Counts{Received: 1}) || len(view.Incidents) != 1 || view.Incidents[0].EndedAt == nil || !view.Incidents[0].EndedAt.Equal(stoppedAt) {
		t.Fatalf("retry changed stopped state: %+v", view)
	}
	var lastAlive, stop int64
	if err := s.store.Read.QueryRowContext(t.Context(), "SELECT last_alive_at_ms,stopped_at_ms FROM message_stats_runs WHERE id=?", s.runID).Scan(&lastAlive, &stop); err != nil || lastAlive != stoppedAt.UnixMilli() || stop != lastAlive {
		t.Fatalf("retry changed stop time: %d/%d, %v", lastAlive, stop, err)
	}
}

func TestQueryCopiesMemoryAfterDatabaseReads(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	clock.add(time.Second)
	receive(s, "bot")
	if err := s.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	s.store.Read.SetMaxOpenConns(1)
	conn, err := s.store.Read.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	waits := s.store.Read.Stats().WaitCount
	type result struct {
		view Response
		err  error
	}
	queried := make(chan result, 1)
	go func() {
		view, err := s.Query(t.Context(), Query{"2026-10-01T00:00:00Z", "2026-10-01T02:00:00Z", "hour"})
		queried <- result{view, err}
	}()
	waitForStatsDBWait(t, s.store.Read, waits)
	clock.add(time.Hour)
	statsUpdateWithin(t, func() {
		receive(s, "bot")
		s.Sent("bot", "onebot11")
		s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true, Connected: true}})
		s.ReloadAdapter("bot")
	})
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	got := <-queried
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.view.Totals != (Counts{2, 1}) || !got.view.AsOf.Equal(clock.now()) || got.view.Connections[0].LastReceivedAt == nil || !got.view.Connections[0].LastReceivedAt.Equal(clock.now()) || len(got.view.Incidents) != 1 || got.view.Incidents[0].EndedAt == nil {
		t.Fatalf("query missed updates during reads: %+v", got.view)
	}
}
