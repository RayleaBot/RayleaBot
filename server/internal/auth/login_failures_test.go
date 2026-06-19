package auth

import (
	"testing"
	"time"
)

func TestLoginFailureTrackerLimitsWithinWindow(t *testing.T) {
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	tracker := NewLoginFailureTracker(func() time.Time {
		return now
	})

	tracker.RecordFailure("127.0.0.1", 2, time.Minute)
	if tracker.IsLimited("127.0.0.1", 2, time.Minute) {
		t.Fatal("single failure should not reach limit")
	}

	tracker.RecordFailure("127.0.0.1", 2, time.Minute)
	if !tracker.IsLimited("127.0.0.1", 2, time.Minute) {
		t.Fatal("second failure should reach limit")
	}

	now = now.Add(time.Minute + time.Second)
	if tracker.IsLimited("127.0.0.1", 2, time.Minute) {
		t.Fatal("expired failures should not keep source limited")
	}
}

func TestLoginFailureTrackerResetClearsFailures(t *testing.T) {
	tracker := NewLoginFailureTracker(nil)

	tracker.RecordFailure("127.0.0.1", 1, time.Minute)
	if !tracker.IsLimited("127.0.0.1", 1, time.Minute) {
		t.Fatal("failure should reach limit")
	}

	tracker.Reset("127.0.0.1")
	if tracker.IsLimited("127.0.0.1", 1, time.Minute) {
		t.Fatal("reset should clear source failures")
	}
}
