package messagestats

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func TestStopHistoryUsesAdjacentIDsAndExclusiveTimeBounds(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	base := clock.now().Add(time.Hour)
	for _, run := range []struct {
		id, start, stop int64
		stopped         bool
	}{
		{10, 0, 5, true}, {20, 10, 20, true}, {40, 15, 17, true},
		{70, 30, 0, false}, {90, 40, 45, true}, {120, 50, 50, true},
		{140, 50, 60, true}, {200, 70, 80, true},
	} {
		stop := sql.NullInt64{Int64: base.Add(time.Duration(run.stop) * time.Minute).UnixMilli(), Valid: run.stopped}
		if _, err := s.store.Write.ExecContext(t.Context(), `INSERT INTO message_stats_runs(id,started_at_ms,last_alive_at_ms,stopped_at_ms) VALUES(?,?,?,?)`, run.id, base.Add(time.Duration(run.start)*time.Minute).UnixMilli(), stop.Int64, stop); err != nil {
			t.Fatal(err)
		}
	}
	q := sqlcgen.New(s.store.Read)
	for _, tc := range []struct {
		start, asOf int64
		want        [][2]int64
	}{
		{10, 60, [][2]int64{{45, 50}, {17, 30}}},
		{49, 60, [][2]int64{{45, 50}}},
		{10, 45, [][2]int64{{17, 30}}},
		{80, 90, nil},
	} {
		rows, err := q.ListMessageStatsStops(t.Context(), sqlcgen.ListMessageStatsStopsParams{
			StartMs: base.Add(time.Duration(tc.start) * time.Minute).UnixMilli(),
			AsOfMs:  sql.NullInt64{Int64: base.Add(time.Duration(tc.asOf) * time.Minute).UnixMilli(), Valid: true},
		})
		if err != nil || len(rows) != len(tc.want) {
			t.Fatalf("window %d/%d: rows=%+v, error=%v", tc.start, tc.asOf, rows, err)
		}
		for index, want := range tc.want {
			if !rows[index].StartedAtMs.Valid || rows[index].StartedAtMs.Int64 != base.Add(time.Duration(want[0])*time.Minute).UnixMilli() || rows[index].EndedAtMs != base.Add(time.Duration(want[1])*time.Minute).UnixMilli() {
				t.Fatalf("window %d/%d: nonadjacent or out-of-bound gap %+v", tc.start, tc.asOf, rows[index])
			}
		}
	}
}

func TestLatestStopHistoryMergesWithPendingOfflineAtLimit(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	s.ObserveAdapters(nil)
	base := clock.now().Add(time.Hour)
	tx, err := s.store.Write.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(t.Context(), `INSERT INTO message_stats_runs(id,started_at_ms,last_alive_at_ms,stopped_at_ms) VALUES(?,?,?,?)`)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 1004; index++ {
		start := base.Add(time.Duration(index) * time.Minute)
		stop := sql.NullInt64{Int64: start.Add(30 * time.Second).UnixMilli(), Valid: index < 1003}
		if _, err := stmt.ExecContext(t.Context(), 100+index*3, start.UnixMilli(), stop.Int64, stop); err != nil {
			_ = stmt.Close()
			t.Fatal(err)
		}
	}
	if err := stmt.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	clock.ms.Store(base.Add(1005 * time.Minute).UnixMilli())
	rows, err := sqlcgen.New(s.store.Read).ListMessageStatsStops(t.Context(), sqlcgen.ListMessageStatsStopsParams{StartMs: base.UnixMilli(), AsOfMs: sql.NullInt64{Int64: clock.now().UnixMilli(), Valid: true}})
	if err != nil || len(rows) != 1001 {
		t.Fatalf("stop history lookahead=%d, %v", len(rows), err)
	}
	view := queryStats(t, s, base.Format(time.RFC3339), base.Add(time.Hour).Format(time.RFC3339), "hour")
	if !view.IncidentsTruncated || len(view.Incidents) != 1000 || !view.Incidents[0].StartedAt.Equal(base.Add(3*time.Minute+30*time.Second)) || !view.Incidents[999].StartedAt.Equal(base.Add(1002*time.Minute+30*time.Second)) {
		t.Fatalf("latest stop history=%d truncated=%v", len(view.Incidents), view.IncidentsTruncated)
	}
	s.ObserveAdapters([]Adapter{{ID: "late", Enabled: true, Connected: true}})
	s.ObserveAdapters([]Adapter{{ID: "late", Enabled: true}})
	clock.add(30 * time.Second)
	view = queryStats(t, s, base.Format(time.RFC3339), base.Add(time.Hour).Format(time.RFC3339), "hour")
	if !view.IncidentsTruncated || len(view.Incidents) != 1000 || !view.Incidents[0].StartedAt.Equal(base.Add(4*time.Minute+30*time.Second)) || view.Incidents[999].Kind != "adapter_offline" || view.Incidents[999].AdapterID != "late" {
		t.Fatalf("combined latest history=%d truncated=%v", len(view.Incidents), view.IncidentsTruncated)
	}
}

func TestSparseHourGroupsKeepConnectionsAndComparisonBoundaries(t *testing.T) {
	s, clock, cfg := newStats(t, "UTC", "2026-09-29T00:00:00Z")
	cfg.Adapters = append(cfg.Adapters, config.AdapterInstance{ID: "configured-empty", Type: "qqofficial"})
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	previous := start.Add(-24 * time.Hour)
	var current, comparison Counts
	for index := 0; index < 64; index++ {
		id := fmt.Sprintf("historic-%03d", index)
		value := int64(index + 1)
		for _, at := range []time.Time{start.Add(time.Duration(index%11) * time.Hour), previous.Add(18 * time.Hour)} {
			if _, err := s.store.Write.ExecContext(t.Context(), `INSERT INTO message_stats_hours(hour_start,adapter_id,received,sent) VALUES(?,?,?,?)`, at.Unix(), id, value, 2*value); err != nil {
				t.Fatal(err)
			}
		}
		current.add(Counts{value, 2 * value})
		if index%3 == 0 {
			if _, err := s.store.Write.ExecContext(t.Context(), `INSERT INTO message_stats_hours(hour_start,adapter_id,received,sent) VALUES(?,?,?,?)`, previous.Add(time.Duration(index%11)*time.Hour).Unix(), id, value, 2*value); err != nil {
				t.Fatal(err)
			}
			comparison.add(Counts{value, 2 * value})
		}
		if _, err := s.store.Write.ExecContext(t.Context(), `INSERT INTO message_stats_adapters(adapter_id,protocol) VALUES(?,'qqofficial')`, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		id string
		at time.Time
	}{{"gap-only", previous.Add(18 * time.Hour)}, {"zero-current", start.Add(time.Hour)}} {
		if _, err := s.store.Write.ExecContext(t.Context(), `INSERT INTO message_stats_hours(hour_start,adapter_id,received,sent) VALUES(?,?,0,0)`, row.at.Unix(), row.id); err != nil {
			t.Fatal(err)
		}
	}
	clock.ms.Store(start.Add(time.Hour).UnixMilli())
	receive(s, "historic-000")
	current.Received++
	clock.ms.Store(start.Add(12 * time.Hour).UnixMilli())
	for _, unit := range []string{"hour", "day"} {
		view := queryStats(t, s, start.Format(time.RFC3339), start.Add(24*time.Hour).Format(time.RFC3339), unit)
		if view.Totals != current || view.Previous == nil || *view.Previous != comparison || len(view.Connections) != 67 {
			t.Fatalf("%s sparse summary=%+v/%+v connections=%d", unit, view.Totals, view.Previous, len(view.Connections))
		}
		if view.Connections[0].AdapterID != "bot" || view.Connections[1].AdapterID != "configured-empty" || view.Connections[66].AdapterID != "zero-current" {
			t.Fatal("connection order or zero-count row changed")
		}
		for index := 0; index < 64; index++ {
			connection := view.Connections[index+2]
			value := int64(index + 1)
			want := Counts{value, 2 * value}
			if index == 0 {
				want.Received++
			}
			var before Counts
			if index%3 == 0 {
				before = Counts{value, 2 * value}
			}
			protocol := "qqofficial"
			if index == 0 {
				protocol = "onebot11"
			}
			if connection.AdapterID != fmt.Sprintf("historic-%03d", index) || connection.Configured || connection.Protocol != protocol || connection.Totals != want || connection.Previous == nil || *connection.Previous != before {
				t.Fatalf("%s sparse connection %d: %+v", unit, index, connection)
			}
		}
	}
}

func TestHourGroupsPreserveScanErrorsAfterValidRows(t *testing.T) {
	for _, period := range []string{"current", "comparison-gap"} {
		for _, field := range []string{"received", "sent"} {
			t.Run(period+"/"+field, func(t *testing.T) {
				s, clock, _ := newStats(t, "UTC", "2026-09-29T00:00:00Z")
				start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
				for index := 0; index < 64; index++ {
					for _, at := range []time.Time{start.Add(-23 * time.Hour), start.Add(time.Hour)} {
						if _, err := s.store.Write.ExecContext(t.Context(), `INSERT INTO message_stats_hours(hour_start,adapter_id,received,sent) VALUES(?,?,1,0)`, at.Unix(), fmt.Sprintf("valid-%03d", index)); err != nil {
							t.Fatal(err)
						}
					}
				}
				at := start.Add(2 * time.Hour)
				if period == "comparison-gap" {
					at = start.Add(-6 * time.Hour)
				}
				received, sent := any(int64(0)), any(int64(0))
				if field == "received" {
					received = 1.5
				} else {
					sent = "not-an-integer"
				}
				if _, err := s.store.Write.ExecContext(t.Context(), `INSERT INTO message_stats_hours(hour_start,adapter_id,received,sent) VALUES(?,'bad',?,?)`, at.Unix(), received, sent); err != nil {
					t.Fatal(err)
				}
				clock.ms.Store(start.Add(12 * time.Hour).UnixMilli())
				receive(s, "bot")
				for _, unit := range []string{"hour", "day"} {
					if _, err := s.Query(t.Context(), Query{start.Format(time.RFC3339), start.Add(24 * time.Hour).Format(time.RFC3339), unit}); err == nil {
						t.Fatalf("%s accepted invalid %s in %s", unit, field, period)
					}
				}
				if _, err := s.store.Write.ExecContext(t.Context(), `DELETE FROM message_stats_hours WHERE adapter_id='bad'`); err != nil {
					t.Fatal(err)
				}
				view := queryStats(t, s, start.Format(time.RFC3339), start.Add(24*time.Hour).Format(time.RFC3339), "day")
				if view.Totals != (Counts{Received: 65}) || view.Previous == nil || *view.Previous != (Counts{Received: 64}) {
					t.Fatal("failed scan changed pending or persisted counts")
				}
			})
		}
	}
}
