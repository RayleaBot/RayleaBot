package adapter

import (
	"math"
	"math/rand"
	"time"
)

type Backoff struct {
	initial time.Duration
	max     time.Duration

	multiplier float64
	jitter     float64
	randFloat  func() float64
}

func NewBackoff(initialSeconds int, multiplier float64, maxSeconds int, jitterRatio float64, randFloat func() float64) *Backoff {
	initial := time.Duration(initialSeconds) * time.Second
	if initial <= 0 {
		initial = time.Second
	}

	maxDelay := time.Duration(maxSeconds) * time.Second
	if maxDelay <= 0 {
		maxDelay = initial
	}
	if maxDelay < initial {
		maxDelay = initial
	}

	if multiplier < 1 {
		multiplier = 1
	}
	if jitterRatio < 0 {
		jitterRatio = 0
	}
	if jitterRatio > 1 {
		jitterRatio = 1
	}
	if randFloat == nil {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		randFloat = rng.Float64
	}

	return &Backoff{
		initial:    initial,
		max:        maxDelay,
		multiplier: multiplier,
		jitter:     jitterRatio,
		randFloat:  randFloat,
	}
}

func (b *Backoff) Duration(attempt int) time.Duration {
	if b == nil {
		return time.Second
	}

	base := float64(b.initial)
	maxDelay := float64(b.max)

	for i := 0; i < attempt; i++ {
		base *= b.multiplier
		if base >= maxDelay {
			base = maxDelay
			break
		}
	}

	jittered := base
	if b.jitter > 0 {
		factor := 1 - b.jitter + (2 * b.jitter * b.randFloat())
		jittered = base * factor
	}

	if jittered < 0 {
		jittered = 0
	}
	if jittered > maxDelay {
		jittered = maxDelay
	}

	return time.Duration(math.Round(jittered))
}
