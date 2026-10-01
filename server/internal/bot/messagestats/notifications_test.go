package messagestats

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func newNotifyingStats(t *testing.T, notify func(Change)) *Service {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	origin := time.Now()
	start := time.Date(2026, 10, 1, 8, 0, 0, 0, time.FixedZone("fixture", 8*60*60))
	s, err := New(t.Context(), Options{
		Store: store, Timezone: "UTC", NotifyChanged: notify,
		Now: func() time.Time { return start.Add(time.Since(origin)) },
		CurrentConfig: func() config.Config {
			return config.Config{Adapters: []config.AdapterInstance{{ID: "bot", Enabled: true}}}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Stop(context.Background()); err != nil {
			t.Error(err)
		}
	})
	synctest.Wait()
	return s
}

func assertNoChange(t *testing.T, notices <-chan Change) {
	t.Helper()
	synctest.Wait()
	select {
	case change := <-notices:
		t.Fatalf("unexpected notification: %+v", change)
	default:
	}
}

func assertChange(t *testing.T, notices <-chan Change, ids ...string) Change {
	t.Helper()
	synctest.Wait()
	select {
	case change := <-notices:
		if !reflect.DeepEqual(change.AdapterIDs, ids) || change.ChangedAt.Location() != time.UTC {
			t.Fatalf("notification = %+v, want UTC and adapters %v", change, ids)
		}
		assertNoChange(t, notices)
		return change
	default:
		t.Fatal("missing notification")
		return Change{}
	}
}

func TestNotificationsCoalesceCountsAndStayIdle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		notices := make(chan Change, 8)
		s := newNotifyingStats(t, func(c Change) { notices <- c })
		s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true, Connected: true}})
		s.Received(chatevent.NormalizedEvent{SourceAdapter: "ignored", EventType: "notice.group"})
		time.Sleep(2 * time.Second)
		assertNoChange(t, notices)
		time.Sleep(100 * time.Millisecond)
		receive(s, "z-bot")
		time.Sleep(500 * time.Millisecond)
		s.Sent("a-bot", "qqofficial")
		receive(s, "z-bot")
		if err := s.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
		time.Sleep(1399 * time.Millisecond)
		assertNoChange(t, notices)
		time.Sleep(time.Millisecond)
		first := assertChange(t, notices, "a-bot", "z-bot")
		if first.ChangedAt.Format(time.RFC3339) != "2026-10-01T00:00:04Z" {
			t.Fatalf("wrong merge time: %s", first.ChangedAt)
		}
		time.Sleep(time.Millisecond)
		s.Sent("a-bot", "qqofficial")
		time.Sleep(1998 * time.Millisecond)
		assertNoChange(t, notices)
		time.Sleep(time.Millisecond)
		second := assertChange(t, notices, "a-bot")
		if second.ChangedAt.Sub(first.ChangedAt) != 2*time.Second {
			t.Fatalf("notification spacing: %s", second.ChangedAt.Sub(first.ChangedAt))
		}
		time.Sleep(10 * time.Second)
		assertNoChange(t, notices)
	})
}

func TestNotificationsObserveOfflineThresholdAndEndWithoutMessages(t *testing.T) {
	for _, path := range []string{"disconnect", "startup", "enable", "reload"} {
		t.Run(path, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				notices := make(chan Change, 8)
				s := newNotifyingStats(t, func(c Change) { notices <- c })
				observe := func(enabled, connected bool) {
					s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: enabled, Connected: connected}})
				}
				threshold := 90 * time.Second
				if path != "startup" {
					observe(true, true)
					time.Sleep(123 * time.Millisecond)
					switch path {
					case "disconnect":
						observe(true, false)
						threshold = 30 * time.Second
					case "enable":
						observe(false, false)
						observe(true, false)
					case "reload":
						s.ReloadAdapter("bot")
					}
				}
				visibleAt := s.timeNow().Add(threshold)
				time.Sleep(threshold - time.Millisecond)
				assertNoChange(t, notices)
				time.Sleep(2 * time.Second)
				change := assertChange(t, notices, "bot")
				if change.ChangedAt.Before(visibleAt) || change.ChangedAt.Sub(visibleAt) > 2*time.Second {
					t.Fatalf("threshold notice at %s, visible at %s", change.ChangedAt, visibleAt)
				}
				observe(true, false)
				time.Sleep(6 * time.Second)
				assertNoChange(t, notices)
				switch path {
				case "disconnect":
					observe(true, true)
				case "startup":
					observe(false, false)
				case "enable":
					s.ObserveAdapters(nil)
				case "reload":
					s.ReloadAdapter("bot")
				}
				endedAt := s.timeNow()
				time.Sleep(2 * time.Second)
				change = assertChange(t, notices, "bot")
				if change.ChangedAt.Sub(endedAt) > 2*time.Second {
					t.Fatal("late interval end notification")
				}
				time.Sleep(6 * time.Second)
				assertNoChange(t, notices)
				if path == "reload" {
					time.Sleep(84 * time.Second)
					assertChange(t, notices, "bot")
				}
			})
		})
	}
}

func TestNotificationsIgnoreShortOfflineIntervals(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		notices := make(chan Change, 8)
		s := newNotifyingStats(t, func(c Change) { notices <- c })
		time.Sleep(89 * time.Second)
		s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true, Connected: true}})
		time.Sleep(2 * time.Second)
		assertNoChange(t, notices)
		s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true}})
		time.Sleep(29 * time.Second)
		s.ObserveAdapters(nil)
		time.Sleep(2 * time.Second)
		assertNoChange(t, notices)
	})
}

func TestNotificationCallbackRunsOutsideLocksAndKeepsNewChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		notices := make(chan Change, 8)
		var s *Service
		s = newNotifyingStats(t, func(c Change) {
			if !s.mu.TryLock() {
				t.Error("notification held mu")
				return
			}
			s.mu.Unlock()
			if !s.ioMu.TryLock() {
				t.Error("notification held ioMu")
				return
			}
			s.ioMu.Unlock()
			notices <- c
			if c.AdapterIDs[0] == "first" {
				s.Sent("second", "onebot11")
			}
		})
		receive(s, "first")
		time.Sleep(2 * time.Second)
		assertChange(t, notices, "first")
		time.Sleep(2 * time.Second)
		assertChange(t, notices, "second")
	})
}

func TestNotificationsDoNotWaitForIOAndStopDiscardsPendingChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		notices := make(chan Change, 8)
		s := newNotifyingStats(t, func(c Change) { notices <- c })
		func() {
			conn, err := s.store.Write.Conn(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			waits := s.store.Write.Stats().WaitCount
			// The flush holds ioMu while waiting for the occupied connection.
			time.Sleep(30 * time.Second)
			synctest.Wait()
			if s.store.Write.Stats().WaitCount == waits {
				t.Fatal("periodic flush did not reach the occupied connection")
			}
			receive(s, "bot")
			time.Sleep(2 * time.Second)
			assertChange(t, notices, "bot")
		}()
		receive(s, "pending")
		if err := s.Stop(t.Context()); err != nil {
			t.Fatal(err)
		}
		receive(s, "after-stop")
		s.Sent("after-stop", "onebot11")
		s.ObserveAdapters(nil)
		s.ReloadAdapter("bot")
		time.Sleep(2 * time.Minute)
		assertNoChange(t, notices)
		var count int
		if err := s.store.Read.QueryRow("SELECT SUM(received+sent) FROM message_stats_hours").Scan(&count); err != nil || count != 2 {
			t.Fatalf("final counts = %d, %v", count, err)
		}
	})
}

func TestStopJoinsInFlightNotificationBeforeFinalFlush(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		s := newNotifyingStats(t, func(Change) { close(entered); <-release })
		receive(s, "bot")
		time.Sleep(2 * time.Second)
		<-entered
		stopped := make(chan error, 1)
		go func() { stopped <- s.Stop(t.Context()) }()
		synctest.Wait()
		select {
		case err := <-stopped:
			t.Fatalf("Stop returned before notifier: %v", err)
		default:
		}
		var rows int
		if err := s.store.Read.QueryRow("SELECT COUNT(*) FROM message_stats_hours").Scan(&rows); err != nil || rows != 0 {
			t.Fatalf("final flush ran before notifier stopped: %d, %v", rows, err)
		}
		close(release)
		if err := <-stopped; err != nil {
			t.Fatal(err)
		}
	})
}

func TestExpiredStopStillJoinsNotifier(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		s := newNotifyingStats(t, func(Change) { close(entered); <-release })
		receive(s, "bot")
		time.Sleep(2 * time.Second)
		<-entered
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		stopped := make(chan error, 1)
		go func() { stopped <- s.Stop(ctx) }()
		synctest.Wait()
		select {
		case err := <-stopped:
			t.Fatalf("expired Stop left a notification in flight: %v", err)
		default:
		}
		close(release)
		if err := <-stopped; err != context.Canceled {
			t.Fatalf("expired Stop error = %v", err)
		}
		time.Sleep(4 * time.Second)
	})
}

func TestOfflineIntervalEndingBetweenNotifierChecksStillNotifies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		notices := make(chan Change, 8)
		s := newNotifyingStats(t, func(c Change) { notices <- c })
		s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true, Connected: true}})
		time.Sleep(time.Second)
		s.ObserveAdapters([]Adapter{{ID: "bot", Enabled: true}})
		time.Sleep(30 * time.Second)
		assertNoChange(t, notices)
		s.ObserveAdapters(nil)
		time.Sleep(time.Second)
		assertChange(t, notices, "bot")
		time.Sleep(4 * time.Second)
		assertNoChange(t, notices)
	})
}
