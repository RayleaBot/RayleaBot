package logging

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestRetentionRunsAtStartupAndPeriodicallyWithCurrentPolicy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		repository := &retentionRepository{}
		stream := NewStream(8)
		stream.SetRepository(repository, 7)
		defer stream.Close()
		stream.Append(Summary{LogID: "fixture", Timestamp: "2026-10-07T00:00:00Z", Level: "info", Message: "fixture"})
		if err := stream.Flush(t.Context()); err != nil {
			t.Fatal(err)
		}
		if len(repository.snapshot()) != 0 {
			t.Fatal("writing a log triggered retention cleanup")
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		startedAt := time.Now()
		go stream.RunRetentionLoop(ctx)
		synctest.Wait()
		if len(repository.snapshot()) != 1 || !repository.snapshot()[0].Equal(startedAt.AddDate(0, 0, -7)) {
			t.Fatalf("startup cutoff: %v", repository.snapshot())
		}
		stream.SetRepository(repository, 3)
		time.Sleep(retentionSweepInterval)
		synctest.Wait()
		if len(repository.snapshot()) != 2 || !repository.snapshot()[1].Equal(time.Now().AddDate(0, 0, -3)) {
			t.Fatalf("periodic cleanup did not use current policy: %v", repository.snapshot())
		}
		cancel()
		synctest.Wait()
		time.Sleep(retentionSweepInterval)
		synctest.Wait()
		if len(repository.snapshot()) != 2 {
			t.Fatal("cleanup continued after cancellation")
		}
	})
}

type retentionRepository struct {
	recordingRepository
	mu      sync.Mutex
	cutoffs []time.Time
}

func (r *retentionRepository) PruneOlderThan(_ context.Context, cutoff time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cutoffs = append(r.cutoffs, cutoff)
	return errors.New("retry on next tick")
}

func (r *retentionRepository) snapshot() []time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.cutoffs)
}
