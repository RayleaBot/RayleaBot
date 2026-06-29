package systemapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/health"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	systemmodel "github.com/RayleaBot/RayleaBot/server/internal/system/model"
)

type diagnosticsTestSystem struct {
	snapshot systemmodel.DiagnosticsSnapshot
}

func (s diagnosticsTestSystem) CurrentReadiness() health.ReadinessReport {
	return health.ReadinessReport{Status: "ready"}
}

func (s diagnosticsTestSystem) DiagnosticsSnapshot(context.Context) systemmodel.DiagnosticsSnapshot {
	return s.snapshot
}

func (s diagnosticsTestSystem) BuildDiagnosticsArchive(context.Context) ([]byte, error) {
	return nil, nil
}

func (s diagnosticsTestSystem) SubmitSystemBackupTask() (string, error) {
	return "", nil
}

func (s diagnosticsTestSystem) ValidateRecoveryConfirmRequest([]string, string) *systemmodel.Error {
	return nil
}

func (s diagnosticsTestSystem) SubmitRecoveryRecheckTask() (string, *systemmodel.Error) {
	return "", nil
}

func (s diagnosticsTestSystem) SubmitRecoveryConfirmTask([]string, string, string) (string, *systemmodel.Error) {
	return "", nil
}

func (s diagnosticsTestSystem) SubmitRuntimeBootstrapTask([]string) (string, error) {
	return "", nil
}

func TestSystemDiagnosticsHTTP(t *testing.T) {
	t.Parallel()

	handler := NewSystemHandlers(diagnosticsTestSystem{
		snapshot: systemmodel.DiagnosticsSnapshot{
			GeneratedAt: "2026-06-25T00:00:00Z",
			Build:       systemmodel.DiagnosticsBuild{CoreVersion: "0.1.0"},
			System:      systemmodel.DiagnosticsSystem{Status: "running", UptimeSeconds: 10},
			Config:      systemmodel.DiagnosticsConfig{SchemaVersion: "2"},
			Secrets:     systemmodel.DiagnosticsSecrets{UnresolvedRefs: []string{}},
			Database: systemmodel.DiagnosticsDatabase{
				SchemaVersion:     "000004",
				AppliedMigrations: []systemmodel.DiagnosticsMigration{},
			},
			Adapter:      systemmodel.DiagnosticsAdapter{State: "connected"},
			Plugins:      systemmodel.DiagnosticsPlugins{},
			Render:       systemmodel.DiagnosticsIssueGroup{Status: "ok", Issues: []health.DiagnosticIssue{}},
			ThirdParty:   systemmodel.DiagnosticsThirdParty{Platforms: []systemmodel.DiagnosticsThirdPartyPlatform{}},
			Dependencies: []systemmodel.DiagnosticsDependency{},
			Filesystem:   []systemmodel.DiagnosticsPathPermission{},
			RecentErrors: []logging.Summary{},
			Issues:       []health.DiagnosticIssue{},
		},
	}).HandleSystemDiagnostics()
	req := httptest.NewRequest(http.MethodGet, "/api/system/diagnostics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["generated_at"] != "2026-06-25T00:00:00Z" {
		t.Fatalf("unexpected diagnostics response: %#v", response)
	}
}
