package messagestats

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/sqlcgen"
)

func TestDayAggregationKeepsOverflowAndPendingCounts(t *testing.T) {
	for _, test := range []struct {
		name, period string
		days         int
		spread       bool
	}{
		{"current", "current", 1, false},
		{"previous", "previous", 1, false},
		{"previous_across_long_window", "previous", 90, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, clock, _ := newStats(t, "UTC", "2026-01-01T00:00:00Z")
			currentStart := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
			length := time.Duration(test.days) * 24 * time.Hour
			for _, period := range []struct {
				name  string
				start time.Time
			}{
				{"previous", currentStart.Add(-length)},
				{"current", currentStart},
			} {
				first := int64(1)
				if period.name == test.period {
					first = math.MaxInt64
				}
				offset := time.Hour
				if test.spread && period.name == "previous" {
					offset = 70 * 24 * time.Hour
				}
				for index, value := range []int64{first, 1} {
					if _, err := s.store.Write.ExecContext(t.Context(),
						"INSERT INTO message_stats_hours(hour_start, adapter_id, received, sent) VALUES (?, 'bot', ?, ?)",
						period.start.Add(time.Duration(index)*offset).Unix(), value, value); err != nil {
						t.Fatal(err)
					}
				}
				clock.ms.Store(period.start.Add(offset + time.Hour).UnixMilli())
				receive(s, "bot")
				s.Sent("bot", "onebot11")
			}
			end := currentStart.Add(length)
			clock.ms.Store(end.UnixMilli())
			view := queryStats(t, s, currentStart.Format(time.RFC3339), end.Format(time.RFC3339), "day")
			current, previous := Counts{3, 3}, Counts{3, 3}
			wrapped := int64(math.MaxInt64)
			wrapped += 2
			if test.period == "current" {
				current = Counts{wrapped, wrapped}
			} else {
				previous = Counts{wrapped, wrapped}
			}
			if view.Totals != current || view.Previous == nil || *view.Previous != previous || len(view.Connections) != 1 || view.Connections[0].Totals != current || *view.Connections[0].Previous != previous {
				t.Fatalf("aggregation changed overflow or duplicated pending counts: %+v", view)
			}
		})
	}
}

func TestDayQueryAggregatesLongWindowsPerAdapter(t *testing.T) {
	const days = 180
	s, clock, _ := newStats(t, "UTC", "2025-01-01T00:00:00Z")
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	length := days * 24 * time.Hour
	end := start.Add(length)
	ids := []string{"bot", "removed-01", "removed-02", "removed-03", "removed-04", "removed-05", "removed-06", "removed-07"}
	tx, err := s.store.Write.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	q := sqlcgen.New(tx)
	for _, period := range []struct {
		start  time.Time
		factor int64
	}{
		{start.Add(-length), 2},
		{start, 3},
	} {
		for day := range days {
			at := period.start.Add((time.Duration(day)*24 + 12) * time.Hour)
			for index, id := range ids {
				value := int64(index+1) * period.factor
				if err := q.AddMessageStatsHour(t.Context(), sqlcgen.AddMessageStatsHourParams{
					HourStart: at.Unix(), AdapterID: id, Received: value, Sent: 2 * value,
				}); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	clock.ms.Store(start.Add(-12 * time.Hour).UnixMilli())
	s.Sent("bot", "onebot11")
	clock.ms.Store(end.Add(-12 * time.Hour).UnixMilli())
	receive(s, "bot")
	clock.ms.Store(end.UnixMilli())
	view := queryStats(t, s, start.Format(time.RFC3339), end.Format(time.RFC3339), "day")
	if len(view.Buckets) != days || len(view.Connections) != len(ids) {
		t.Fatalf("long window shape = %d buckets, %d connections", len(view.Buckets), len(view.Connections))
	}
	var wantTotals, wantPrevious Counts
	for index, connection := range view.Connections {
		current, previous := int64(index+1)*3, int64(index+1)*2
		wantCurrent := Counts{current * days, 2 * current * days}
		wantComparison := Counts{previous * days, 2 * previous * days}
		if index == 0 {
			wantCurrent.Received++
			wantComparison.Sent++
		}
		if connection.AdapterID != ids[index] || connection.Totals != wantCurrent || connection.Previous == nil || *connection.Previous != wantComparison {
			t.Fatalf("long window mixed connection counts: %+v", connection)
		}
		for day := range days {
			want := current
			if index == 0 && day == days-1 {
				want++
			}
			if connection.Received[day] != want || connection.Sent[day] != 2*current {
				t.Fatalf("connection %s day %d = %d/%d", connection.AdapterID, day, connection.Received[day], connection.Sent[day])
			}
		}
		wantTotals.add(wantCurrent)
		wantPrevious.add(wantComparison)
	}
	if view.Totals != wantTotals || view.Previous == nil || *view.Previous != wantPrevious {
		t.Fatalf("long window totals = %+v/%+v", view.Totals, view.Previous)
	}
}

func TestDayQueryPropagatesCancellation(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	clock.add(time.Hour)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := s.Query(ctx, Query{"2026-10-01T00:00:00Z", "2026-10-02T00:00:00Z", "day"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("query cancellation = %v", err)
	}
}

func TestDayAggregationPreservesUpsertOverflowErrorsAcrossFullReadRange(t *testing.T) {
	for _, period := range []struct{ name, at string }{
		{"current", "2026-10-01T06:00:00Z"},
		{"previous", "2026-09-30T06:00:00Z"},
		{"undisplayed_gap", "2026-09-30T18:00:00Z"},
	} {
		for _, field := range []string{"received", "sent"} {
			t.Run(period.name+"/"+field, func(t *testing.T) {
				s, clock, _ := newStats(t, "UTC", "2026-09-29T00:00:00Z")
				at, _ := time.Parse(time.RFC3339, period.at)
				q := sqlcgen.New(s.store.Write)
				for _, value := range []int64{math.MaxInt64, 1} {
					count := sqlcgen.AddMessageStatsHourParams{HourStart: at.Unix(), AdapterID: "bot"}
					if field == "received" {
						count.Received = value
					} else {
						count.Sent = value
					}
					if err := q.AddMessageStatsHour(t.Context(), count); err != nil {
						t.Fatal(err)
					}
				}
				var receivedType, sentType string
				if err := s.store.Read.QueryRowContext(t.Context(), "SELECT typeof(received), typeof(sent) FROM message_stats_hours").Scan(&receivedType, &sentType); err != nil {
					t.Fatal(err)
				}
				if field == "received" && receivedType != "real" || field == "sent" && sentType != "real" {
					t.Fatalf("overflowed storage types = %s/%s", receivedType, sentType)
				}
				clock.set("2026-10-01T12:00:00Z")
				for _, granularity := range []string{"hour", "day"} {
					if _, err := s.Query(t.Context(), Query{"2026-10-01T00:00:00Z", "2026-10-02T00:00:00Z", granularity}); err == nil {
						t.Fatalf("%s query accepted an unscannable %s count in %s", granularity, field, period.name)
					}
				}
			})
		}
	}
}
