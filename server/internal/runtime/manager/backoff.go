package manager

import (
	"math"
	"time"
)

// DefaultMaxCrashRetries is the maximum number of consecutive crash-restart
// attempts before the runtime enters dead_letter state.
const DefaultMaxCrashRetries = 5

// CrashBackoff computes the next retry delay using capped exponential backoff.
//
//	delay = min(initialSeconds * 2^(crashCount-1), maxSeconds)
//
// crashCount must be >= 1. initialSeconds and maxSeconds are clamped to
// sensible minimums if they are zero or negative.
func CrashBackoff(crashCount, initialSeconds, maxSeconds int) time.Duration {
	if initialSeconds <= 0 {
		initialSeconds = 2
	}
	if maxSeconds <= 0 {
		maxSeconds = 60
	}
	if crashCount <= 0 {
		crashCount = 1
	}

	delay := float64(initialSeconds) * math.Pow(2, float64(crashCount-1))
	if delay > float64(maxSeconds) {
		delay = float64(maxSeconds)
	}

	return time.Duration(delay) * time.Second
}
