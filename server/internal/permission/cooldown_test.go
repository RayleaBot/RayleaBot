package permission

import (
	"testing"
	"time"
)

func TestAllowWithinLimit(t *testing.T) {
	t.Parallel()

	tracker := NewCooldownTracker(
		RateLimit{Count: 3, Window: time.Minute},
		RateLimit{Count: 3, Window: time.Minute},
	)

	for i := range 3 {
		if !tracker.Allow("user:u1") {
			t.Fatalf("call %d should be allowed within limit", i+1)
		}
	}
}

func TestDenyWhenLimitExceeded(t *testing.T) {
	t.Parallel()

	tracker := NewCooldownTracker(
		RateLimit{Count: 2, Window: time.Minute},
		RateLimit{Count: 2, Window: time.Minute},
	)

	tracker.Allow("user:u1")
	tracker.Allow("user:u1")

	if tracker.Allow("user:u1") {
		t.Fatal("third call should be denied when limit is 2")
	}
}

func TestAllowAgainAfterWindowExpires(t *testing.T) {
	t.Parallel()

	tracker := NewCooldownTracker(
		RateLimit{Count: 1, Window: 50 * time.Millisecond},
		RateLimit{Count: 1, Window: 50 * time.Millisecond},
	)

	if !tracker.Allow("user:u1") {
		t.Fatal("first call should be allowed")
	}
	if tracker.Allow("user:u1") {
		t.Fatal("second call should be denied within window")
	}

	// Wait for window to expire.
	time.Sleep(60 * time.Millisecond)

	if !tracker.Allow("user:u1") {
		t.Fatal("call after window expiry should be allowed")
	}
}
