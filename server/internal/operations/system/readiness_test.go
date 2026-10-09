package system

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/health"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestReadinessReportsMissingInboundToken(t *testing.T) {
	for _, tt := range []struct {
		name           string
		host           string
		disabled       bool
		settings       config.OneBotConfig
		failedDatabase bool
		wantStatus     string
		wantIssue      bool
	}{
		{name: "reverse websocket", host: "0.0.0.0", settings: config.OneBotConfig{ReverseWS: config.OneBotTransportConfig{Enabled: true}}, wantStatus: "degraded", wantIssue: true},
		{name: "webhook preserves failed", host: "::", settings: config.OneBotConfig{Webhook: config.OneBotTransportConfig{Enabled: true}}, failedDatabase: true, wantStatus: "failed", wantIssue: true},
		{name: "loopback", host: "127.0.0.1", settings: config.OneBotConfig{Webhook: config.OneBotTransportConfig{Enabled: true}}, wantStatus: "ready"},
		{name: "disabled adapter", host: "0.0.0.0", disabled: true, settings: config.OneBotConfig{ReverseWS: config.OneBotTransportConfig{Enabled: true}}, wantStatus: "ready"},
		{name: "outbound only", host: "0.0.0.0", settings: config.OneBotConfig{ForwardWS: config.OneBotTransportConfig{Enabled: true}, HTTPAPI: config.OneBotTransportConfig{Enabled: true}}, wantStatus: "ready"},
		{name: "resolved token", host: "0.0.0.0", settings: config.OneBotConfig{Webhook: config.OneBotTransportConfig{Enabled: true, AccessToken: "fixture-only-token"}}, wantStatus: "ready"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{Adapters: []config.AdapterInstance{{ID: "fixture", Type: config.AdapterTypeOneBot11, Enabled: !tt.disabled, OneBot11: &tt.settings}}}
			store := openReadinessStore(t)
			if tt.failedDatabase {
				if err := store.Read.Close(); err != nil {
					t.Fatal(err)
				}
			}
			service, err := New(Deps{
				ListenHost:     tt.host,
				CurrentConfig:  func() config.Config { return cfg },
				CurrentSummary: func() config.Summary { return config.Summary{} },
				Plugins:        plugincatalog.New(nil), Auth: readinessAuthState(true), Storage: store,
			})
			if err != nil {
				t.Fatal(err)
			}
			// A pending host edit must not change the listener policy until restart.
			cfg.Server.Host = "localhost"
			report := service.CurrentReadiness()
			code := errorcodes.DiagnosticAdapterInboundTokenMissing
			if report.Status != tt.wantStatus || slices.Contains(report.ReasonCodes, code) != tt.wantIssue {
				t.Fatalf("readiness = %#v", report)
			}
			for _, issues := range [][]health.DiagnosticIssue{report.Issues, service.DiagnosticsSnapshot(t.Context()).Issues} {
				index := slices.IndexFunc(issues, func(issue health.DiagnosticIssue) bool { return issue.Code == code })
				if (index >= 0) != tt.wantIssue {
					t.Fatalf("inbound token issue = %#v", issues)
				}
				if index >= 0 && (issues[index].Severity != "warning" || issues[index].Remediation == "") {
					t.Fatalf("missing actionable warning: %#v", issues[index])
				}
			}
			if tt.wantIssue {
				cfg.Adapters[0].OneBot11 = &config.OneBotConfig{
					ReverseWS: config.OneBotTransportConfig{Enabled: true, AccessToken: "fixture-only-token"},
					Webhook:   config.OneBotTransportConfig{Enabled: true, AccessToken: "fixture-only-token"},
				}
				if slices.Contains(service.CurrentReadiness().ReasonCodes, code) {
					t.Fatal("warning remained after effective tokens were set")
				}
			}
		})
	}
}

func TestCurrentReadinessDoesNotRequireOneBotAdapter(t *testing.T) {
	t.Parallel()

	service, err := New(Deps{
		Plugins:        plugincatalog.New(nil),
		CurrentConfig:  func() config.Config { return config.Config{} },
		CurrentSummary: func() config.Summary { return config.Summary{} },
		Auth:           initializedReadinessAuth(t),
		Storage:        openReadinessStore(t),
	})

	if err != nil {
		t.Fatal(err)
	}
	report := service.CurrentReadiness()
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

func openReadinessStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

type readinessAuthState bool

func (s readinessAuthState) IsBootstrapped() bool { return bool(s) }

type readinessRenderer struct {
	issues []health.DiagnosticIssue
}

func (r readinessRenderer) Diagnostics() []health.DiagnosticIssue { return r.issues }
func (r readinessRenderer) RefreshBrowserPath(string)             {}

func TestReadinessOmitsUnevaluatedChecks(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		auth   AuthBootstrapState
		status string
		code   string
		reason string
	}{
		{"auth unavailable", nil, "failed", errorcodes.DiagnosticAuthUnavailable, "管理认证服务不可用"},
		{"setup required", readinessAuthState(false), "setup_required", errorcodes.DiagnosticSetupRequired, "需要先完成管理员初始化"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			service := &Service{auth: tt.auth}
			report := service.CurrentReadiness()
			if report.Status != tt.status || report.Reason != tt.reason || len(report.Checks) != 0 {
				t.Fatalf("unexpected early readiness: %#v", report)
			}
			if len(report.Issues) != 1 || report.Issues[0].Code != tt.code || report.Issues[0].Summary != tt.reason {
				t.Fatalf("unexpected early readiness issue: %#v", report.Issues)
			}
		})
	}
}

func TestReadinessProbesDatabaseOnEveryEvaluation(t *testing.T) {
	t.Parallel()
	store := openReadinessStore(t)
	service := &Service{auth: readinessAuthState(true), storage: store, startupRuntimes: newStartupRuntimeStates(nil)}
	if report := service.CurrentReadiness(); report.Status != "ready" || report.Checks["database"] != "ok" {
		t.Fatalf("open database readiness = %#v", report)
	}
	if err := store.Read.Close(); err != nil {
		t.Fatal(err)
	}
	issue := startupFailureIssue("ffmpeg", errors.New("fixture failure"))
	service.setStartupRuntimeState("ffmpeg", StartupRuntimePhaseFailed, &issue)
	service.renderer = readinessRenderer{issues: []health.DiagnosticIssue{startupFailureIssue("chromium", nil)}}
	report := service.CurrentReadiness()
	if report.Status != "failed" || report.Checks["database"] != "unavailable" || report.Reason != "数据库不可用" {
		t.Fatalf("closed database readiness = %#v", report)
	}
	want := health.DiagnosticIssue{
		Code:        errorcodes.DiagnosticDatabasePingFailed,
		Severity:    "error",
		Summary:     "数据库不可用",
		Remediation: "请检查数据库文件、磁盘空间与文件权限，然后重启服务。",
	}
	if len(report.Issues) != 3 || !reflect.DeepEqual(report.Issues[0], want) {
		t.Fatalf("database issue must precede runtime and render issues: %#v", report.Issues)
	}
	if !reflect.DeepEqual(report.ReasonCodes, []string{errorcodes.DiagnosticDatabasePingFailed, errorcodes.PlatformResourceMissing}) {
		t.Fatalf("reason codes = %#v", report.ReasonCodes)
	}
}

func TestReadinessDatabaseProbeTimesOut(t *testing.T) {
	t.Parallel()
	store := openReadinessStore(t)
	store.Read.SetMaxOpenConns(1)
	conn, err := store.Read.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	service := &Service{storage: store}
	started := time.Now()
	if service.databaseAvailable() {
		t.Fatal("probe succeeded while the only connection was held")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("one-second probe exceeded scheduling allowance: %v", elapsed)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if !service.databaseAvailable() {
		t.Fatal("probe did not recover after releasing the connection")
	}
}

func TestReadinessUsesManagedRuntimeState(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		phase  StartupRuntimePhase
		issue  bool
		check  string
		status string
	}{
		{"ready", StartupRuntimePhaseReady, false, "ok", "ready"},
		{"not required", StartupRuntimePhaseNotRequired, false, "ok", "ready"},
		{"preparing", StartupRuntimePhasePending, false, "preparing", "ready"},
		{"failed", StartupRuntimePhaseFailed, true, "resource_missing", "degraded"},
		{"failed without issue", StartupRuntimePhaseFailed, false, "resource_missing", "degraded"},
		{"missing state", "", false, "resource_missing", "degraded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := &Service{auth: readinessAuthState(true), storage: openReadinessStore(t)}
			var issue *health.DiagnosticIssue
			if tt.issue {
				stored := startupFailureIssue("ffmpeg", errors.New("fixture failure"))
				issue = &stored
			}
			if tt.phase != "" {
				service.setStartupRuntimeState("ffmpeg", tt.phase, issue)
			}
			report := service.CurrentReadiness()
			if report.Status != tt.status || report.Checks["runtime"] != tt.check || report.Checks["render"] != "ok" {
				t.Fatalf("runtime readiness = %#v", report)
			}
			if tt.status == "ready" {
				if len(report.Issues) != 0 || report.Reason != "" || len(report.ReasonCodes) != 0 {
					t.Fatalf("ready or preparing runtime reported an issue: %#v", report)
				}
				return
			}
			if len(report.Issues) != 1 || report.Reason != report.Issues[0].Summary ||
				!reflect.DeepEqual(report.ReasonCodes, []string{errorcodes.PlatformResourceMissing}) ||
				!reflect.DeepEqual(report.Issues[0].RuntimeResources, []string{"ffmpeg"}) {
				t.Fatalf("missing actionable FFmpeg issue: %#v", report)
			}
			if issue != nil && !reflect.DeepEqual(report.Issues[0], *issue) {
				t.Fatalf("stored issue changed: %#v", report.Issues[0])
			}
		})
	}
}
