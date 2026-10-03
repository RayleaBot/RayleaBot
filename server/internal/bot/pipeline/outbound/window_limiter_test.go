package outbound

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestPruneWindowRecordsRetainsCutoffAndExpiresEarlierRecords(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	cutoff := now.Add(-5 * time.Second)
	records := []time.Time{cutoff.Add(-time.Nanosecond), cutoff, now}
	records = pruneWindowRecords(records, now, 5*time.Second)
	if len(records) != 2 || !records[0].Equal(cutoff) || !records[1].Equal(now) {
		t.Fatalf("records at cutoff = %v, want [%v %v]", records, cutoff, now)
	}
	records = pruneWindowRecords(records, now.Add(time.Nanosecond), 5*time.Second)
	if len(records) != 1 || !records[0].Equal(now) {
		t.Fatalf("records after cutoff = %v, want [%v]", records, now)
	}
	records = pruneWindowRecords(records, now.Add(6*time.Second), 5*time.Second)
	if len(records) != 0 {
		t.Fatalf("expired records = %v, want none", records)
	}
	if records = pruneWindowRecords(append(records, now), now, 0); len(records) != 0 {
		t.Fatalf("records with disabled window = %v, want none", records)
	}
}

func TestPruneWindowRecordsReleasesSparseBurstStorage(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	cutoff := now.Add(-5 * time.Second)
	records := make([]time.Time, 1024)
	for index := range records {
		records[index] = cutoff.Add(-time.Second)
	}
	records[len(records)-2], records[len(records)-1] = cutoff, now
	burstCapacity := cap(records)
	records = pruneWindowRecords(records, now, 5*time.Second)
	if len(records) != 2 || !records[0].Equal(cutoff) || !records[1].Equal(now) {
		t.Fatalf("records after burst expiry = %v, want [%v %v]", records, cutoff, now)
	}
	if cap(records) >= burstCapacity/4 {
		t.Fatalf("retained capacity = %d for %d live records, want burst storage released", cap(records), len(records))
	}
	records = pruneWindowRecords(records, now.Add(time.Nanosecond), 5*time.Second)
	if len(records) != 1 || !records[0].Equal(now) {
		t.Fatalf("records after cutoff = %v, want [%v]", records, now)
	}
}

func TestWindowLimiterPreservesFIFOAfterConfigChange(t *testing.T) {
	limiter := newWindowLimiter(time.Now, config.RateLimit{Count: 1, Window: time.Hour})
	if err := limiter.Wait(t.Context(), "target"); err != nil {
		t.Fatal(err)
	}
	results := make(chan windowWaitResult, 4)
	for id := range 3 {
		go waitForWindowResult(limiter, t.Context(), id, results)
		awaitWindowQueueLength(t, limiter, 1+id)
	}

	limiter.SetLimit(config.RateLimit{Count: 2, Window: time.Hour})
	assertWindowResult(t, results, 0, nil)
	go waitForWindowResult(limiter, t.Context(), 3, results)
	awaitWindowQueueLength(t, limiter, 3)
	for id := 1; id <= 3; id++ {
		limiter.SetLimit(config.RateLimit{Count: id + 2, Window: time.Hour})
		assertWindowResult(t, results, id, nil)
	}
}

func TestWindowLimiterCancellationUnblocksFollowingWaiters(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		cancelled int
		remaining []int
	}{
		{name: "Head", cancelled: 0, remaining: []int{1, 2}},
		{name: "Middle", cancelled: 1, remaining: []int{0, 2}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			limiter := newWindowLimiter(time.Now, config.RateLimit{Count: 1, Window: time.Hour})
			if err := limiter.Wait(t.Context(), "target"); err != nil {
				t.Fatal(err)
			}
			results := make(chan windowWaitResult, 3)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			for id := range 3 {
				waitContext := t.Context()
				if id == scenario.cancelled {
					waitContext = ctx
				}
				go waitForWindowResult(limiter, waitContext, id, results)
				awaitWindowQueueLength(t, limiter, 1+id)
			}
			cancel()
			assertWindowResult(t, results, scenario.cancelled, context.Canceled)
			awaitWindowQueueLength(t, limiter, 2)
			for index, id := range scenario.remaining {
				limiter.SetLimit(config.RateLimit{Count: index + 2, Window: time.Hour})
				assertWindowResult(t, results, id, nil)
			}
		})
	}
}

func TestWindowLimiterCancelledContextDoesNotConsumeQuota(t *testing.T) {
	limiter := newWindowLimiter(time.Now, config.RateLimit{Count: 1, Window: time.Hour})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for range 20 {
		if err := limiter.Wait(ctx, "target"); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled Wait() error = %v, want context.Canceled", err)
		}
	}
	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := limiter.Wait(ctx, "target"); err != nil {
		t.Fatalf("cancelled requests consumed quota: %v", err)
	}
}

func TestWindowLimiterCancelledHeadCannotReserveAfterConfigWake(t *testing.T) {
	limiter := newWindowLimiter(time.Now, config.RateLimit{Count: 1, Window: time.Hour})
	if err := limiter.Wait(t.Context(), "target"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	waiter, err := limiter.reserveOrEnqueue(ctx, "target")
	if err != nil || waiter == nil {
		t.Fatalf("reserveOrEnqueue() = %v, %v, want queued waiter", waiter, err)
	}
	cancel()
	limiter.SetLimit(config.RateLimit{Count: 2, Window: time.Hour})
	if _, _, err := limiter.tryReserve(ctx, "target", waiter); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled head reservation error = %v, want context.Canceled", err)
	}
	limiter.cancelWaiter("target", waiter)
	ctx, cancel = context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := limiter.Wait(ctx, "target"); err != nil {
		t.Fatalf("cancelled head consumed quota after config wake: %v", err)
	}
}

type windowWaitResult struct {
	id  int
	err error
}

func waitForWindowResult(limiter *windowLimiter, ctx context.Context, id int, results chan<- windowWaitResult) {
	results <- windowWaitResult{id: id, err: limiter.Wait(ctx, "target")}
}

func assertWindowResult(t *testing.T, results <-chan windowWaitResult, id int, wantErr error) {
	t.Helper()
	select {
	case result := <-results:
		if result.id != id || !errors.Is(result.err, wantErr) {
			t.Fatalf("wait result = {%d, %v}, want {%d, %v}", result.id, result.err, id, wantErr)
		}
	case <-time.After(time.Second):
		t.Fatalf("waiter %d did not finish", id)
	}
}

func awaitWindowQueueLength(t *testing.T, limiter *windowLimiter, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		limiter.mu.Lock()
		queued := 0
		if state := limiter.windows["target"]; state != nil {
			queued = len(state.queue)
		}
		limiter.mu.Unlock()
		if queued == count {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("queued waiters = %d, want %d", queued, count)
		}
		time.Sleep(time.Millisecond)
	}
}
