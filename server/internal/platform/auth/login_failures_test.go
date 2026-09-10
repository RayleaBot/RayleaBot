package auth

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoginFailureTrackerReservesAttemptsWithinWindow(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	tracker := NewLoginFailureTracker(func() time.Time {
		return now
	})

	if !tracker.Reserve("127.0.0.1", 2, time.Minute) {
		t.Fatal("first attempt should be reserved")
	}
	if !tracker.Reserve("127.0.0.1", 2, time.Minute) {
		t.Fatal("second attempt should be reserved")
	}
	if tracker.Reserve("127.0.0.1", 2, time.Minute) {
		t.Fatal("third attempt should be rejected")
	}

	now = now.Add(time.Minute + time.Second)
	if !tracker.Reserve("127.0.0.1", 2, time.Minute) {
		t.Fatal("expired reservations should not keep source limited")
	}
}

func TestLoginFailureTrackerResetClearsFailures(t *testing.T) {
	tracker := NewLoginFailureTracker(nil)

	if !tracker.Reserve("127.0.0.1", 1, time.Minute) {
		t.Fatal("attempt should be reserved")
	}
	if tracker.Reserve("127.0.0.1", 1, time.Minute) {
		t.Fatal("limit should reject a second reservation")
	}

	tracker.Reset("127.0.0.1")
	if !tracker.Reserve("127.0.0.1", 1, time.Minute) {
		t.Fatal("reset should clear source reservations")
	}
}

func TestLoginFailureTrackerConcurrentReservationDoesNotBurstPastLimit(t *testing.T) {
	tracker := NewLoginFailureTracker(nil)
	var admitted atomic.Int32
	var workers sync.WaitGroup
	workers.Add(6)
	for range 6 {
		go func() {
			defer workers.Done()
			if tracker.Reserve("127.0.0.1", 5, time.Minute) {
				admitted.Add(1)
			}
		}()
	}
	workers.Wait()

	if admitted.Load() != 5 {
		t.Fatalf("admitted attempts = %d, want 5", admitted.Load())
	}
}
