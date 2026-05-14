package pluginwebhook

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEvaluateReplayProtectionRejectsDuplicateEventID(t *testing.T) {
	t.Parallel()

	fixedNow := time.Unix(1_700_000_000, 0)
	svc := &Service{
		dedup: newReplayCache(),
		now:   func() time.Time { return fixedNow },
	}
	cfg := ReplayProtection{
		TimestampHeader:  "X-Raylea-Timestamp",
		EventIDHeader:    "X-Raylea-Event-Id",
		ToleranceSeconds: 300,
		Enforce:          true,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/repo-watcher/github", nil)
	req.Header.Set(cfg.TimestampHeader, "1700000000")
	req.Header.Set(cfg.EventIDHeader, "evt-dup-1")

	first := svc.evaluateReplayProtection("repo-watcher", "github", cfg, req)
	if first.reject {
		t.Fatalf("first request must be accepted, got %+v", first)
	}
	// Peek-then-commit: the caller is expected to commit the dedup entry
	// only after authentication succeeds, so simulate that here before
	// evaluating the second request.
	if first.dedupKey == "" {
		t.Fatal("first decision must surface a dedup key")
	}
	svc.dedup.commit(first.dedupKey, fixedNow)

	second := svc.evaluateReplayProtection("repo-watcher", "github", cfg, req)
	if !second.reject || second.code != "plugin.webhook_replay_rejected" {
		t.Fatalf("expected replay rejection on duplicate event id, got %+v", second)
	}
}

// TestEvaluateReplayProtectionDoesNotPoisonOnFailedAuth simulates the
// finding where an unauthenticated client could write the dedup cache by
// exercising evaluateReplayProtection without committing. The genuine
// retry must still be accepted.
func TestEvaluateReplayProtectionDoesNotPoisonOnFailedAuth(t *testing.T) {
	t.Parallel()

	fixedNow := time.Unix(1_700_000_000, 0)
	svc := &Service{
		dedup: newReplayCache(),
		now:   func() time.Time { return fixedNow },
	}
	cfg := ReplayProtection{
		TimestampHeader:  "X-Raylea-Timestamp",
		EventIDHeader:    "X-Raylea-Event-Id",
		ToleranceSeconds: 300,
		Enforce:          true,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/repo-watcher/github", nil)
	req.Header.Set(cfg.TimestampHeader, "1700000000")
	req.Header.Set(cfg.EventIDHeader, "evt-poison-attempt")

	first := svc.evaluateReplayProtection("repo-watcher", "github", cfg, req)
	if first.reject {
		t.Fatalf("attacker request must pass replay window itself, got %+v", first)
	}
	// Simulate failed authentication: the caller must NOT commit.

	second := svc.evaluateReplayProtection("repo-watcher", "github", cfg, req)
	if second.reject {
		t.Fatalf("genuine retry must be accepted after failed-auth peek, got %+v", second)
	}
}

func TestEvaluateReplayProtectionRejectsTimestampOutsideWindow(t *testing.T) {
	t.Parallel()

	fixedNow := time.Unix(1_700_000_000, 0)
	svc := &Service{
		dedup: newReplayCache(),
		now:   func() time.Time { return fixedNow },
	}
	cfg := ReplayProtection{
		TimestampHeader:  "X-Raylea-Timestamp",
		EventIDHeader:    "X-Raylea-Event-Id",
		ToleranceSeconds: 60,
		Enforce:          true,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/repo-watcher/github", nil)
	req.Header.Set(cfg.TimestampHeader, "1699999000")
	req.Header.Set(cfg.EventIDHeader, "evt-stale-1")

	decision := svc.evaluateReplayProtection("repo-watcher", "github", cfg, req)
	if !decision.reject || decision.code != "plugin.webhook_timestamp_skew" {
		t.Fatalf("expected timestamp skew rejection, got %+v", decision)
	}
}

func TestEvaluateReplayProtectionGraceModeLogsButAccepts(t *testing.T) {
	t.Parallel()

	fixedNow := time.Unix(1_700_000_000, 0)
	metrics := &recordingMetrics{}
	svc := &Service{
		dedup:   newReplayCache(),
		now:     func() time.Time { return fixedNow },
		metrics: metrics,
	}
	cfg := ReplayProtection{
		TimestampHeader:  "X-Raylea-Timestamp",
		EventIDHeader:    "X-Raylea-Event-Id",
		ToleranceSeconds: 300,
		Enforce:          false,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/repo-watcher/github", nil)

	decision := svc.evaluateReplayProtection("repo-watcher", "github", cfg, req)
	if decision.reject {
		t.Fatalf("grace mode must accept requests missing headers, got reject")
	}
	if got := metrics.counts["grace_observed"]; got != 1 {
		t.Fatalf("expected one grace_observed metric, got %d", got)
	}
	if got := metrics.counts["rejected"]; got != 0 {
		t.Fatalf("grace mode must not record rejection, got %d", got)
	}
}

func TestEvaluateReplayProtectionEnforceMissingHeaders(t *testing.T) {
	t.Parallel()

	fixedNow := time.Unix(1_700_000_000, 0)
	svc := &Service{
		dedup: newReplayCache(),
		now:   func() time.Time { return fixedNow },
	}
	cfg := ReplayProtection{
		TimestampHeader:  "X-Raylea-Timestamp",
		EventIDHeader:    "X-Raylea-Event-Id",
		ToleranceSeconds: 300,
		Enforce:          true,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/repo-watcher/github", nil)

	decision := svc.evaluateReplayProtection("repo-watcher", "github", cfg, req)
	if !decision.reject || decision.code != "plugin.webhook_replay_rejected" {
		t.Fatalf("enforce mode must reject missing headers, got %+v", decision)
	}
}

type recordingMetrics struct {
	counts map[string]int
}

func (m *recordingMetrics) IncReplayObserved(outcome string) {
	if m.counts == nil {
		m.counts = make(map[string]int)
	}
	m.counts[outcome]++
}
