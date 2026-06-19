package backoff

import (
	"testing"
	"time"
)

func TestBackoffDurationWithoutJitter(t *testing.T) {
	t.Parallel()

	backoff := NewWithDurations(time.Second, 2, 10*time.Second, 0, func() float64 { return 0.5 })

	if got := backoff.Duration(0); got != time.Second {
		t.Fatalf("attempt 0: got %s want 1s", got)
	}
	if got := backoff.Duration(1); got != 2*time.Second {
		t.Fatalf("attempt 1: got %s want 2s", got)
	}
	if got := backoff.Duration(2); got != 4*time.Second {
		t.Fatalf("attempt 2: got %s want 4s", got)
	}
}

func TestBackoffDurationCapsAtMaximum(t *testing.T) {
	t.Parallel()

	backoff := NewWithDurations(time.Second, 2, 3*time.Second, 0, func() float64 { return 0.5 })

	if got := backoff.Duration(5); got != 3*time.Second {
		t.Fatalf("got %s want 3s", got)
	}
}

func TestBackoffDurationAppliesDeterministicJitter(t *testing.T) {
	t.Parallel()

	backoff := NewWithDurations(time.Second, 2, 10*time.Second, 0.2, func() float64 { return 0.75 })

	got := backoff.Duration(1)
	want := time.Duration(2200) * time.Millisecond
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestBackoffJitterStaysWithinBounds(t *testing.T) {
	t.Parallel()

	backoff := NewWithDurations(time.Second, 2, 5*time.Second, 0.25, func() float64 { return 1.0 })

	got := backoff.Duration(2)
	if got > 5*time.Second {
		t.Fatalf("got %s want <= 5s", got)
	}
	if got < 3*time.Second {
		t.Fatalf("got %s want >= 3s", got)
	}
}
