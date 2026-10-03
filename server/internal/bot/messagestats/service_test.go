package messagestats

import (
	"context"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type testClock struct{ ms atomic.Int64 }

func (c *testClock) now() time.Time { return time.UnixMilli(c.ms.Load()).UTC() }
func (c *testClock) set(raw string) {
	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		panic(err)
	}
	c.ms.Store(at.UnixMilli())
}
func (c *testClock) add(d time.Duration) { c.ms.Add(d.Milliseconds()) }

func newStats(t *testing.T, zone, start string) (*Service, *testClock, *config.Config) {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	clock := &testClock{}
	clock.set(start)
	cfg := &config.Config{Adapters: []config.AdapterInstance{{ID: "bot", Type: "onebot11", Enabled: true}}}
	s, err := New(t.Context(), Options{Store: store, Now: clock.now, Timezone: zone, CurrentConfig: func() config.Config { return *cfg }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.loopCancel(); <-s.loopDone })
	return s, clock, cfg
}
func receive(s *Service, id string) {
	s.Received(chatevent.NormalizedEvent{SourceAdapter: id, SourceProtocol: "onebot11", Kind: chatevent.EventKindMessageText, EventType: "message.group"})
}
func queryStats(t *testing.T, s *Service, start, end, unit string) Response {
	t.Helper()
	result, err := s.Query(t.Context(), Query{start, end, unit})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestHourlyPendingCountsPersistExactlyOnce(t *testing.T) {
	s, clock, _ := newStats(t, "Asia/Shanghai", "2026-10-01T01:11:00Z")
	receive(s, "bot")
	s.Sent("bot", "onebot11")
	clock.set("2026-10-01T02:00:00.001Z")
	receive(s, "bot")
	before := queryStats(t, s, "2026-10-01T00:30:00Z", "2026-10-01T03:01:00Z", "hour")
	if len(before.Buckets) != 2 || before.StartAt.Hour() != 0 || before.EndAt.Hour() != 4 || before.Previous != nil || before.Totals != (Counts{2, 1}) || !reflect.DeepEqual(before.Connections[0].Received, []int64{1, 1}) {
		t.Fatalf("unexpected view: %+v", before)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := s.Flush(canceled); err == nil {
		t.Fatal("canceled flush succeeded")
	}
	for range 2 {
		if err := s.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	after := queryStats(t, s, "2026-10-01T00:30:00Z", "2026-10-01T03:01:00Z", "hour")
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("flush changed response: before=%+v after=%+v", before, after)
	}
	if err := s.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	clock.add(time.Minute)
	resumed, err := New(t.Context(), Options{Store: s.store, Now: clock.now, Timezone: "Asia/Shanghai", CurrentConfig: s.config})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resumed.loopCancel(); <-resumed.loopDone })
	result := queryStats(t, resumed, "2026-10-01T00:30:00Z", "2026-10-01T03:01:00Z", "hour")
	if result.Totals != before.Totals || !result.TrackingStartedAt.Equal(before.TrackingStartedAt) || !result.Connections[0].LastReceivedAt.Equal(*before.Connections[0].LastReceivedAt) {
		t.Fatalf("restart lost counters/metadata: %+v", result)
	}
}

func TestQueryCombinesPersistedAndPendingHours(t *testing.T) {
	for _, tc := range []struct {
		name, zone, start, end, unit string
	}{
		{"hour", "UTC", "2026-05-02T05:00:00Z", "2026-05-02T06:00:00Z", "hour"},
		{"day", "America/New_York", "2026-03-08T05:00:00Z", "2026-03-09T04:00:00Z", "day"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, clock, _ := newStats(t, tc.zone, "2026-01-01T00:00:00Z")
			start, _ := time.Parse(time.RFC3339, tc.start)
			end, _ := time.Parse(time.RFC3339, tc.end)
			previous := start.Add(-end.Sub(start))
			for batch := range 2 {
				for _, at := range []time.Time{previous, start} {
					clock.ms.Store(at.Add(30 * time.Minute).UnixMilli())
					for _, id := range []string{"bot", "removed"} {
						receive(s, id)
						if batch == 1 {
							s.Sent(id, "onebot11")
						}
					}
				}
				if batch == 0 {
					if err := s.Flush(t.Context()); err != nil {
						t.Fatal(err)
					}
				}
			}
			clock.ms.Store(end.UnixMilli())
			before := queryStats(t, s, tc.start, tc.end, tc.unit)
			if before.Totals != (Counts{4, 2}) || before.Previous == nil || *before.Previous != (Counts{4, 2}) || len(before.Connections) != 2 {
				t.Fatalf("lost persisted or pending counts: %+v", before)
			}
			for _, connection := range before.Connections {
				if connection.Totals != (Counts{2, 1}) || connection.Previous == nil || *connection.Previous != (Counts{2, 1}) || !reflect.DeepEqual(connection.Received, []int64{2}) || !reflect.DeepEqual(connection.Sent, []int64{1}) {
					t.Fatalf("mixed connection or hour aggregates: %+v", connection)
				}
			}
			if err := s.Flush(t.Context()); err != nil {
				t.Fatal(err)
			}
			if after := queryStats(t, s, tc.start, tc.end, tc.unit); !reflect.DeepEqual(before, after) {
				t.Fatalf("flush changed combined counts: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestReceivedKinds(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	for _, kind := range []string{chatevent.FamilyMessageText, chatevent.FamilyMessage, chatevent.FamilyMessageSent, chatevent.FamilyNotice, chatevent.FamilyRequest, chatevent.FamilyMeta} {
		for _, eventType := range []string{"message.group", "message.private", "message_sent.group", "notice.group"} {
			s.Received(chatevent.NormalizedEvent{Kind: chatevent.EventKind("onebot11", kind), EventType: eventType, SourceAdapter: "bot", SourceProtocol: "onebot11"})
		}
	}
	clock.add(time.Second)
	if got := queryStats(t, s, "2026-10-01T00:00:00Z", "2026-10-01T01:00:00Z", "hour").Totals.Received; got != 4 {
		t.Fatalf("received=%d", got)
	}
}

func TestStopPersistsTheFinalUnflushedBatch(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	clock.add(time.Second)
	receive(s, "bot")
	s.Sent("bot", "onebot11")
	if err := s.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	var received, sent int64
	if err := s.store.Read.QueryRow("SELECT received,sent FROM message_stats_hours WHERE adapter_id='bot'").Scan(&received, &sent); err != nil || received != 1 || sent != 1 {
		t.Fatalf("final counters = %d/%d, %v", received, sent, err)
	}
	var lastAlive, stopped int64
	if err := s.store.Read.QueryRow("SELECT last_alive_at_ms,stopped_at_ms FROM message_stats_runs WHERE id=?", s.runID).Scan(&lastAlive, &stopped); err != nil || lastAlive != clock.now().UnixMilli() || stopped != lastAlive {
		t.Fatalf("final run timestamps = %d/%d, %v", lastAlive, stopped, err)
	}
	if err := s.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	view := queryStats(t, s, "2026-10-01T00:00:00Z", "2026-10-01T01:00:00Z", "hour")
	if view.Totals != (Counts{1, 1}) {
		t.Fatalf("repeat shutdown duplicated batch: %+v", view.Totals)
	}
}

func TestDayAggregationCalendarBoundaries(t *testing.T) {
	for _, tc := range []struct {
		zone, start, end string
		hours            int
	}{
		{"Asia/Shanghai", "2026-09-30T16:00:00Z", "2026-10-01T16:00:00Z", 24},
		{"Asia/Kolkata", "2026-09-30T18:30:00Z", "2026-10-01T18:30:00Z", 24},
		{"Asia/Kathmandu", "2026-09-30T18:15:00Z", "2026-10-01T18:15:00Z", 24},
		{"America/New_York", "2026-03-08T05:00:00Z", "2026-03-09T04:00:00Z", 23},
		{"America/New_York", "2026-11-01T04:00:00Z", "2026-11-02T05:00:00Z", 25},
		{"America/Sao_Paulo", "2018-11-04T03:00:00Z", "2018-11-05T02:00:00Z", 23},
		{"America/Havana", "2026-11-01T04:00:00Z", "2026-11-02T05:00:00Z", 25},
		{"Pacific/Apia", "2011-12-29T10:00:00Z", "2011-12-30T10:00:00Z", 24},
	} {
		t.Run(tc.zone+tc.start, func(t *testing.T) {
			s, clock, _ := newStats(t, tc.zone, "2010-01-01T00:00:00Z")
			start, _ := time.Parse(time.RFC3339, tc.start)
			end, _ := time.Parse(time.RFC3339, tc.end)
			for hour := start.Truncate(time.Hour).Add(-time.Hour); !hour.After(end.Add(time.Hour)); hour = hour.Add(time.Hour) {
				clock.ms.Store(hour.UnixMilli())
				receive(s, "bot")
			}
			if err := s.Flush(t.Context()); err != nil {
				t.Fatal(err)
			}
			view := queryStats(t, s, tc.start, tc.end, "day")
			if len(view.Buckets) != 1 || !view.Buckets[0].Equal(start) || view.Totals.Received != int64(tc.hours) {
				t.Fatalf("wrong local day: %+v", view)
			}
		})
	}
}

func TestPreviousWindowAndRemovedAdapterOrdering(t *testing.T) {
	s, clock, cfg := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	clock.set("2026-10-01T08:10:00Z")
	receive(s, "z-old")
	receive(s, "bot")
	clock.set("2026-10-01T09:10:00Z")
	receive(s, "outside")
	clock.set("2026-10-01T10:10:00Z")
	receive(s, "a-old")
	receive(s, "bot")
	cfg.Adapters = append(cfg.Adapters, config.AdapterInstance{ID: "zero", Type: "qqofficial"})
	clock.set("2026-10-01T10:30:00Z")
	view := queryStats(t, s, "2026-10-01T10:00:00Z", "2026-10-01T12:00:00Z", "hour")
	if view.Previous == nil || view.Previous.Received != 2 || view.Totals.Received != 2 {
		t.Fatalf("comparison=%+v", view)
	}
	var ids []string
	for _, c := range view.Connections {
		ids = append(ids, c.AdapterID)
		if c.Previous == nil {
			t.Fatal("missing connection comparison")
		}
	}
	if !reflect.DeepEqual(ids, []string{"bot", "zero", "a-old", "z-old"}) {
		t.Fatalf("order=%v", ids)
	}
	if view.Connections[2].Configured || view.Connections[3].Configured || view.Connections[3].Protocol != "onebot11" {
		t.Fatal("removed metadata missing")
	}
	empty := queryStats(t, s, "2026-10-02T10:00:00Z", "2026-10-02T12:00:00Z", "hour")
	if len(empty.Buckets) != 0 || len(empty.Connections) != 2 || empty.Connections[0].LastReceivedAt == nil {
		t.Fatalf("future view=%+v", empty)
	}
	early := queryStats(t, s, "2026-10-01T01:00:00Z", "2026-10-01T03:00:00Z", "hour")
	if early.Previous != nil || early.Connections[0].Previous != nil {
		t.Fatal("comparison predates tracking")
	}
}

func TestAlignmentValidationAndLimits(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Kolkata")
	for _, q := range []Query{{}, {"bad", "2026-10-01T00:00:00Z", "hour"}, {"2026-10-01T00:00:00Z", "2026-10-01T00:00:00Z", "hour"}, {"2026-10-02T00:00:00Z", "2026-10-01T00:00:00Z", "hour"}, {"2026-10-01T00:00:00Z", "2026-10-02T00:00:00Z", "week"}, {"0001-01-01T00:00:00Z", "9999-01-01T00:00:00Z", "day"}} {
		if _, err := align(q, location); err != ErrInvalidRequest {
			t.Fatalf("accepted %+v: %v", q, err)
		}
	}
	for _, unit := range []string{"hour", "day"} {
		limit := 744
		if unit == "day" {
			limit = 732
		}
		start := floor(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), unit, location)
		end := start
		for range limit {
			end = nextBucket(end, unit, location)
		}
		q := Query{start.Format(time.RFC3339), end.Format(time.RFC3339), unit}
		r, err := align(q, location)
		if err != nil || len(r.buckets) != limit {
			t.Fatalf("limit rejected %s: %v", unit, err)
		}
		q.EndAt = end.Add(time.Second).Format(time.RFC3339)
		if _, err := align(q, location); err != ErrInvalidRequest {
			t.Fatal("limit overflow accepted")
		}
	}
}

func TestConcurrentCountingFlushAndQuery(t *testing.T) {
	s, clock, _ := newStats(t, "UTC", "2026-10-01T00:00:00Z")
	clock.add(time.Second)
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for range 100 {
				receive(s, "bot")
				s.Sent("bot", "onebot11")
			}
		})
	}
	workers.Go(func() {
		for range 10 {
			if err := s.Flush(t.Context()); err != nil {
				t.Error(err)
			}
		}
	})
	workers.Go(func() {
		var previous int64
		for range 20 {
			v, err := s.Query(t.Context(), Query{"2026-10-01T00:00:00Z", "2026-10-01T01:00:00Z", "hour"})
			if err != nil || v.Totals.Received < previous {
				t.Errorf("non-monotonic query: %v %+v", err, v)
			}
			previous = v.Totals.Received
		}
	})
	workers.Wait()
	if got := queryStats(t, s, "2026-10-01T00:00:00Z", "2026-10-01T01:00:00Z", "hour").Totals; got != (Counts{800, 800}) {
		t.Fatalf("lost/duplicated counts: %+v", got)
	}
}
