package outbound

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"testing"
	"time"
)

func TestMessageCircuitBreakerOpensHalfOpensAndRecoversPerTarget(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC)
	breaker := newMessageCircuitBreaker(func() time.Time { return now }, 30*time.Second, 3)
	target := MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "100"}
	otherTarget := MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "200"}
	sendErr := &chatevent.SendError{Code: errorcodes.AdapterSendFailed, Message: "fixture failure"}

	for attempt := 0; attempt < 2; attempt++ {
		breaker.Record(target, sendErr)
		if err := breaker.Allow(target); err != nil {
			t.Fatalf("circuit opened after %d failures: %v", attempt+1, err)
		}
	}
	breaker.Record(target, sendErr)
	assertCircuitOpen(t, breaker.Allow(target))
	if err := breaker.Allow(otherTarget); err != nil {
		t.Fatalf("other target inherited circuit state: %v", err)
	}

	now = now.Add(30 * time.Second)
	if err := breaker.Allow(target); err != nil {
		t.Fatalf("half-open probe was rejected: %v", err)
	}
	assertCircuitOpen(t, breaker.Allow(target))
	breaker.Record(target, nil)
	if err := breaker.Allow(target); err != nil {
		t.Fatalf("successful probe did not close circuit: %v", err)
	}
}

func TestMessageCircuitBreakerReleasesCanceledHalfOpenProbe(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC)
	breaker := newMessageCircuitBreaker(func() time.Time { return now }, 30*time.Second, 1)
	target := MessageLimitRequest{PluginID: "weather", TargetType: "group", TargetID: "100"}
	breaker.Record(target, errors.New("fixture failure"))
	now = now.Add(30 * time.Second)
	if err := breaker.Allow(target); err != nil {
		t.Fatalf("first half-open probe was rejected: %v", err)
	}
	breaker.Record(target, context.Canceled)
	if err := breaker.Allow(target); err != nil {
		t.Fatalf("canceled half-open probe left the circuit permanently occupied: %v", err)
	}
}

func TestMessageCircuitBreakerFailedProbeReopensCooldown(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 23, 8, 0, 0, 0, time.UTC)
	breaker := newMessageCircuitBreaker(func() time.Time { return now }, 10*time.Second, 1)
	target := MessageLimitRequest{TargetType: "private", TargetID: "100"}
	sendErr := &chatevent.SendError{Code: errorcodes.AdapterSendFailed, Message: "fixture failure"}
	breaker.Record(target, sendErr)
	now = now.Add(10 * time.Second)
	if err := breaker.Allow(target); err != nil {
		t.Fatalf("probe was rejected: %v", err)
	}
	breaker.Record(target, sendErr)
	assertCircuitOpen(t, breaker.Allow(target))
	now = now.Add(9 * time.Second)
	assertCircuitOpen(t, breaker.Allow(target))
}

func assertCircuitOpen(t *testing.T, err error) {
	t.Helper()
	var adapterErr *chatevent.SendError
	if !errors.As(err, &adapterErr) || adapterErr.Code != errorcodes.AdapterSendFailed {
		t.Fatalf("error = %#v, want adapter.send_failed circuit-open error", err)
	}
}
