package app

import (
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestCurrentReadinessDoesNotRequireOneBotAdapter(t *testing.T) {
	t.Parallel()

	app := newTestAppState(config.Config{}, nil)
	app.systemService = newSystemService(systemServiceDeps{
		state: app.state,
		auth:  initializedReadinessAuth(t),
	})

	report := app.systemService.CurrentReadiness()
	if report.Status != "ready" {
		t.Fatalf("readiness status = %q, want ready", report.Status)
	}
	if report.Reason != "" {
		t.Fatalf("readiness reason = %q, want empty", report.Reason)
	}
	if len(report.ReasonCodes) != 0 {
		t.Fatalf("readiness reason codes = %#v, want empty", report.ReasonCodes)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("readiness issues = %#v, want empty", report.Issues)
	}
	if _, ok := report.Checks["adapter"]; ok {
		t.Fatalf("readiness checks contain adapter: %#v", report.Checks)
	}
	wantChecks := map[string]string{
		"config":   "ok",
		"database": "ok",
		"runtime":  "ok",
		"render":   "ok",
	}
	if !reflect.DeepEqual(report.Checks, wantChecks) {
		t.Fatalf("readiness checks = %#v, want %#v", report.Checks, wantChecks)
	}
}

func initializedReadinessAuth(t *testing.T) *auth.Manager {
	t.Helper()

	manager, err := auth.NewManager(auth.Config{
		SessionTTLDays: 7,
		SlidingRenewal: true,
		MaxSessions:    3,
	})
	if err != nil {
		t.Fatalf("create auth manager: %v", err)
	}
	if _, _, err := manager.Bootstrap("admin", "fixture-only-secret"); err != nil {
		t.Fatalf("bootstrap auth manager: %v", err)
	}
	return manager
}
