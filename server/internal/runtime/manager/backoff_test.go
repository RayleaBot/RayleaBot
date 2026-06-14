package manager

import (
	"testing"
	"time"
)

func TestCrashBackoff_ExponentialGrowth(t *testing.T) {
	initial := 2
	max := 60

	cases := []struct {
		crashCount int
		want       time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 32 * time.Second},
		{6, 60 * time.Second}, // capped
		{7, 60 * time.Second}, // capped
	}

	for _, tc := range cases {
		got := CrashBackoff(tc.crashCount, initial, max)
		if got != tc.want {
			t.Errorf("CrashBackoff(%d, %d, %d) = %v, want %v",
				tc.crashCount, initial, max, got, tc.want)
		}
	}
}

func TestCrashBackoff_ZeroInputsUseSafeDefaults(t *testing.T) {
	got := CrashBackoff(0, 0, 0)
	if got < 1*time.Second {
		t.Errorf("CrashBackoff(0,0,0) = %v, want >= 1s", got)
	}
}

func TestCrashBackoff_NegativeInputsUseSafeDefaults(t *testing.T) {
	got := CrashBackoff(-1, -5, -10)
	if got < 1*time.Second {
		t.Errorf("CrashBackoff(-1,-5,-10) = %v, want >= 1s", got)
	}
}
