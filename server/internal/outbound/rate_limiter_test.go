package outbound

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestMessageRateLimiterDelaysPluginMessagesUntilWindowAllows(t *testing.T) {
	limiter := NewMessageRateLimiter(config.Config{
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "1/20ms",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
		},
	})

	if err := limiter.Wait(context.Background(), MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "100"}); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}

	startedAt := time.Now()
	if err := limiter.Wait(context.Background(), MessageLimitRequest{PluginID: "weather", TargetType: "private", TargetID: "200"}); err != nil {
		t.Fatalf("second Wait() error = %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed < 15*time.Millisecond {
		t.Fatalf("second Wait() elapsed = %s, want plugin window delay", elapsed)
	}
}

func TestMessageRateLimiterKeepsTargetsIndependent(t *testing.T) {
	limiter := NewMessageRateLimiter(config.Config{
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "100/1s",
			RateLimitPerTarget:    "1/1h",
			CircuitBreakerSeconds: 1,
		},
	})

	if err := limiter.Wait(context.Background(), MessageLimitRequest{PluginID: "a", TargetType: "group", TargetID: "100"}); err != nil {
		t.Fatalf("first target Wait() error = %v", err)
	}
	if err := limiter.Wait(context.Background(), MessageLimitRequest{PluginID: "b", TargetType: "group", TargetID: "200"}); err != nil {
		t.Fatalf("different target Wait() error = %v", err)
	}
}

func TestMessageRateLimiterReturnsPlatformRateLimitedAfterWaitLimit(t *testing.T) {
	limiter := NewMessageRateLimiter(config.Config{
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "100/1s",
			RateLimitPerTarget:    "1/1h",
			CircuitBreakerSeconds: 1,
		},
	})
	limiter.maxWait = 20 * time.Millisecond

	if err := limiter.Wait(context.Background(), MessageLimitRequest{TargetType: "group", TargetID: "100"}); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}

	err := limiter.Wait(context.Background(), MessageLimitRequest{TargetType: "group", TargetID: "100"})
	if err == nil {
		t.Fatal("second Wait() error = nil, want platform.rate_limited")
	}
	var adapterErr *adapter.Error
	if !errors.As(err, &adapterErr) || adapterErr.Code != "platform.rate_limited" {
		t.Fatalf("error = %#v, want platform.rate_limited", err)
	}
}

func TestMessageRateLimiterApplyConfigTakesEffect(t *testing.T) {
	limiter := NewMessageRateLimiter(config.Config{
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "1/1h",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
		},
	})

	if err := limiter.Wait(context.Background(), MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "100"}); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}

	limiter.ApplyConfig(config.Config{
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "2/1h",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
		},
	})

	if err := limiter.Wait(context.Background(), MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "101"}); err != nil {
		t.Fatalf("updated Wait() error = %v", err)
	}
}

func TestMessageRateLimiterApplyConfigWakesQueuedMessages(t *testing.T) {
	limiter := NewMessageRateLimiter(config.Config{
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "1/1h",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
		},
	})

	if err := limiter.Wait(context.Background(), MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "100"}); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- limiter.Wait(context.Background(), MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "101"})
	}()

	select {
	case err := <-done:
		t.Fatalf("queued Wait() finished before config changed: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	limiter.ApplyConfig(config.Config{
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "2/1h",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
		},
	})

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("queued Wait() error = %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("queued Wait() did not observe updated rate limit")
	}
}
