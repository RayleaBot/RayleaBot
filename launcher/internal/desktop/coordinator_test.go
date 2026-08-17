package desktop

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type testServiceHost struct {
	confirmExternal       bool
	service               *Service
	closeResponse         error
	snapshotEmits         int
	trayUpdates           int
	exitConfirmEmits      int
	respondToClose        bool
	externalStopResponses []bool
}

func (h *testServiceHost) Emit(name string, _ any) {
	if name == "launcher:snapshot" {
		h.snapshotEmits++
	}
	if name == "launcher:show-exit-confirm" {
		h.exitConfirmEmits++
		if h.respondToClose && h.service != nil {
			h.closeResponse = h.service.CloseConfirmResponse(LauncherCloseConfirmResponse{Action: "cancel"})
		}
	}
}

func (h *testServiceHost) SetTrayState(TrayMenuState)                              { h.trayUpdates++ }
func (*testServiceHost) OpenURL(string) error                                      { return nil }
func (*testServiceHost) OpenDirectory(string) error                                { return nil }
func (h *testServiceHost) ConfirmExternalServiceStop() bool                        { return h.confirmExternal }
func (*testServiceHost) ChooseDirectory(string, string) (string, error)            { return "", nil }
func (*testServiceHost) ChooseFile(string, string, string, string) (string, error) { return "", nil }
func (*testServiceHost) Minimise()                                                 {}
func (*testServiceHost) ToggleMaximise()                                           {}
func (*testServiceHost) IsMaximised() bool                                         { return false }
func (*testServiceHost) HideWindow()                                               {}
func (*testServiceHost) SetThemeMode(string)                                       {}
func (h *testServiceHost) ResolveExternalServiceStop(confirmed bool) {
	h.externalStopResponses = append(h.externalStopResponses, confirmed)
}
func (*testServiceHost) HasPendingExternalServiceStop() bool { return false }
func (*testServiceHost) Quit()                               {}

func TestReadinessAndRecoveryAcceptOpaqueContractObjects(t *testing.T) {
	readiness := JSONObject{"status": "degraded", "recovery_summary": JSONObject{"status": "blocked"}}
	status := JSONObject{"status": "ready", "recovery_summary": JSONObject{"status": "compatible"}}
	if readinessStatus(readiness) != "degraded" {
		t.Fatalf("readinessStatus() = %q", readinessStatus(readiness))
	}
	recovery, ok := recoveryFromPayload(status, readiness, nil).(JSONObject)
	if !ok || recovery["status"] != "compatible" {
		t.Fatalf("recoveryFromPayload() = %#v", recovery)
	}
}

func TestTrayStateTracksServiceLifecycle(t *testing.T) {
	snapshot := defaultSnapshot()
	snapshot.Server.Health = JSONObject{"status": "ok"}
	snapshot.Server.Readiness = JSONObject{"status": "ready"}
	snapshot.Launcher.ProcessLifecycle = "running"
	snapshot.Launcher.ProcessOwnership = "launcher_managed"
	state := trayState(snapshot)
	if state.TrayStatusSummary != "运行中" || state.TrayServiceAction != "stop" || !state.CanOpenWebUI {
		t.Fatalf("trayState() = %#v", state)
	}
}

func TestTrayStateKeepsStopActionForUnhealthyManagedProcess(t *testing.T) {
	snapshot := defaultSnapshot()
	snapshot.Launcher.ProcessLifecycle = "running"
	snapshot.Launcher.ProcessOwnership = "launcher_managed"
	state := trayState(snapshot)
	if state.TrayServiceAction != "stop" || !state.CanRunTrayServiceAction || state.CanOpenWebUI {
		t.Fatalf("trayState() = %#v", state)
	}
}

func TestGetPlatformPreservesDesktopPlatformAndArchitecture(t *testing.T) {
	platform := runtime.GOOS
	if platform == "windows" {
		platform = "win32"
	}
	architecture := runtime.GOARCH
	if architecture == "amd64" {
		architecture = "x64"
	} else if architecture == "386" {
		architecture = "ia32"
	}
	want := platform + "-" + architecture
	if got := (&Service{}).GetPlatform(); got != want {
		t.Fatalf("GetPlatform() = %q, want %q", got, want)
	}
}

func TestProcessControllerRecordsBoundedStructuredDiagnostics(t *testing.T) {
	controller := NewProcessController("")
	if diagnostics := controller.RecentStderr(); diagnostics == nil || len(diagnostics) != 0 {
		t.Fatalf("empty RecentStderr() = %#v, want a non-nil empty slice", diagnostics)
	}
	controller.recordStructuredOutput(`{"level":"ERROR","msg":"runtime failed","err":"boom","error_code":"runtime.failed","component":"bootstrap"}`)
	controller.recordStructuredOutput(`{"component":"runtime_prepare","resource_kind":"chromium","label":"Chromium","status":"running","progress":50,"summary":"downloading"}`)
	if got := controller.RecentStderr(); len(got) != 1 || got[0] != "runtime failed：boom（错误代码：runtime.failed；组件：bootstrap）" {
		t.Fatalf("RecentStderr() = %#v", got)
	}
	progress := controller.RuntimePrepare()
	if progress == nil || !progress.Active || progress.CurrentKind != "chromium" || len(progress.Resources) != 1 {
		t.Fatalf("RuntimePrepare() = %#v", progress)
	}
	for index := 0; index < maxRecentDiagnostics+5; index++ {
		controller.recordDiagnostic("next")
	}
	if len(controller.RecentStderr()) != maxRecentDiagnostics {
		t.Fatalf("RecentStderr() length = %d", len(controller.RecentStderr()))
	}
}

func TestProcessControllerCapturesPlainTextStartupFailuresFromStdout(t *testing.T) {
	controller := NewProcessController("")
	controller.SetWorkdir(t.TempDir())
	controller.consumeOutput("stdout", strings.NewReader("listen tcp 127.0.0.1:8080: bind: address already in use\nordinary output\n"))

	diagnostics := controller.RecentStderr()
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0], "address already in use") {
		t.Fatalf("stdout diagnostics = %#v", diagnostics)
	}
}

func TestProcessExitReasonIncludesExitCode(t *testing.T) {
	if os.Getenv("RAYLEA_TEST_PROCESS_EXIT") == "1" {
		os.Exit(7)
	}
	command := exec.Command(os.Args[0], "-test.run=TestProcessExitReasonIncludesExitCode")
	command.Env = append(os.Environ(), "RAYLEA_TEST_PROCESS_EXIT=1")
	if err := command.Run(); err == nil {
		t.Fatal("helper process unexpectedly succeeded")
	}
	if reason := processExitReason(command.ProcessState); !strings.Contains(reason, "7") {
		t.Fatalf("process exit reason = %q", reason)
	}
}

func TestLifecyclePublishPreservesConcurrentReleaseState(t *testing.T) {
	coordinator := &Coordinator{snapshot: defaultSnapshot()}
	release := ReleaseCheckSnapshot{Status: "update_available", LatestVersion: "0.2.0"}
	coordinator.publishRelease(release)

	staleLifecycle := defaultSnapshot()
	staleLifecycle.Launcher.StatusHint = "已刷新服务状态。"
	coordinator.publish(staleLifecycle)

	snapshot := coordinator.Snapshot()
	if snapshot.Launcher.ReleaseCheck.Status != release.Status || snapshot.Launcher.ReleaseCheck.LatestVersion != release.LatestVersion {
		t.Fatalf("release state was overwritten: %#v", snapshot.Launcher.ReleaseCheck)
	}
}

func TestSnapshotCopiesNestedMutableState(t *testing.T) {
	progress := 25.0
	snapshot := defaultSnapshot()
	snapshot.Server.Readiness = JSONObject{
		"status": "degraded",
		"issues": []any{map[string]any{"code": "runtime.not_ready"}},
	}
	snapshot.Launcher.Settings.AdvancedOverrides = &LauncherAdvancedOverrides{Workdir: "custom"}
	snapshot.Launcher.RuntimePrepare = &RuntimePrepareSnapshot{
		Resources: []RuntimePrepareResourceProgress{{Kind: "chromium", Progress: &progress}},
	}
	coordinator := &Coordinator{snapshot: snapshot}

	clone := coordinator.Snapshot()
	clone.Server.Readiness.(JSONObject)["status"] = "ready"
	clone.Server.Readiness.(JSONObject)["issues"].([]any)[0].(map[string]any)["code"] = "changed"
	clone.Launcher.Settings.AdvancedOverrides.Workdir = "changed"
	*clone.Launcher.RuntimePrepare.Resources[0].Progress = 100

	current := coordinator.Snapshot()
	if readinessStatus(current.Server.Readiness) != "degraded" {
		t.Fatalf("readiness was mutated through a snapshot copy: %#v", current.Server.Readiness)
	}
	if current.Launcher.Settings.AdvancedOverrides.Workdir != "custom" {
		t.Fatalf("settings overrides were mutated through a snapshot copy: %#v", current.Launcher.Settings)
	}
	if got := *current.Launcher.RuntimePrepare.Resources[0].Progress; got != 25 {
		t.Fatalf("runtime progress was mutated through a snapshot copy: %v", got)
	}
}

func TestPublishSkipsDuplicateSnapshotsAndTrayRebuilds(t *testing.T) {
	host := &testServiceHost{}
	coordinator := &Coordinator{snapshot: defaultSnapshot(), host: host}

	coordinator.publish(defaultSnapshot())
	if host.snapshotEmits != 0 || host.trayUpdates != 0 {
		t.Fatalf("unchanged publish emitted snapshot=%d tray=%d", host.snapshotEmits, host.trayUpdates)
	}

	changed := defaultSnapshot()
	changed.Launcher.StatusHint = "状态已刷新。"
	coordinator.publish(changed)
	coordinator.publish(changed)
	coordinator.publishRelease(ReleaseCheckSnapshot{Status: "update_available", LatestVersion: "0.2.0"})
	coordinator.publishRelease(ReleaseCheckSnapshot{Status: "update_available", LatestVersion: "0.2.0"})
	if host.snapshotEmits != 2 || host.trayUpdates != 1 {
		t.Fatalf("deduplicated publish emitted snapshot=%d tray=%d", host.snapshotEmits, host.trayUpdates)
	}
}

func TestLauncherLogsKeepTheDocumentedDirectoryLayout(t *testing.T) {
	workdir := t.TempDir()
	controller := NewProcessController("")
	controller.SetWorkdir(workdir)
	controller.WriteLauncherLog("启动器日志", "")

	matches, err := filepath.Glob(filepath.Join(workdir, "logs", "launcher", "*.log"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("launcher log files = %#v, %v", matches, err)
	}
	payload, err := os.ReadFile(matches[0])
	if err != nil || len(payload) == 0 {
		t.Fatalf("launcher log payload = %q, %v", payload, err)
	}
}

func TestProcessStartClearsCredentialsWhenPreparingWorkdirFails(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(workdir, []byte("occupied"), 0o600); err != nil {
		t.Fatalf("write workdir blocker: %v", err)
	}
	controller := NewProcessController("")
	err := controller.Start(LauncherResolvedSettings{Workdir: workdir, ServerExecutablePath: filepath.Join(root, "missing-server")})
	if err == nil {
		t.Fatal("Start() unexpectedly succeeded")
	}
	if controller.SetupToken() != "" || controller.ControlToken() != "" {
		t.Fatal("Start() retained credentials after a preparation failure")
	}
}

func TestReplaceEnvironmentValuesRemovesStaleCredentialCopies(t *testing.T) {
	environment := replaceEnvironmentValues([]string{
		"PATH=/usr/bin",
		"RAYLEA_SETUP_TOKEN=stale-setup",
		"raylea_launcher_control_token=stale-control",
		"OTHER=value=with-equals",
	}, map[string]string{
		"RAYLEA_SETUP_TOKEN":            "fresh-setup",
		"RAYLEA_LAUNCHER_CONTROL_TOKEN": "fresh-control",
	})

	joined := strings.Join(environment, "\n")
	if strings.Contains(joined, "stale-") {
		t.Fatalf("environment retained stale credentials: %q", joined)
	}
	for _, expected := range []string{
		"RAYLEA_SETUP_TOKEN=fresh-setup",
		"RAYLEA_LAUNCHER_CONTROL_TOKEN=fresh-control",
		"OTHER=value=with-equals",
	} {
		if strings.Count(joined, expected) != 1 {
			t.Fatalf("environment = %q, want exactly one %q", joined, expected)
		}
	}
}

func TestExternalShutdownFailureRemainsVisibleAndDoesNotForceKill(t *testing.T) {
	shutdownCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/healthz":
			response.WriteHeader(http.StatusOK)
		case "/readyz":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"status":"ready"}`))
		case "/api/launcher/shutdown":
			shutdownCalls++
			response.Header().Set("Content-Type", "application/json")
			response.WriteHeader(http.StatusForbidden)
			_, _ = response.Write([]byte(`{"error":{"code":"launcher.control_required","message":"control token required"}}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	host, portText, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatalf("split server address: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}
	root := t.TempDir()
	serverName := "raylea-server"
	if runtime.GOOS == "windows" {
		serverName += ".exe"
	}
	serverExecutable := filepath.Join(root, serverName)
	configPath := filepath.Join(root, "config", "user.yaml")
	writeTestFile(t, serverExecutable, "test executable")
	writeTestFile(t, configPath, "server:\n  host: "+host+"\n  port: "+portText+"\n")

	hostBridge := &testServiceHost{confirmExternal: true}
	coordinator := NewCoordinator(root, "", 0, hostBridge)
	coordinator.settings = LauncherSettings{InstallationRoot: root, CloseBehavior: closeAsk}
	coordinator.initialized = true
	coordinator.process.SetWorkdir(root)
	if err := coordinator.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if shutdownCalls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", shutdownCalls)
	}
	snapshot := coordinator.Snapshot()
	if !strings.Contains(snapshot.Launcher.LastLocalError, "control token required") || snapshot.Launcher.ProcessOwnership != "external" {
		t.Fatalf("snapshot after rejected shutdown = %#v", snapshot.Launcher)
	}
	connection, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), time.Second)
	if err != nil {
		t.Fatalf("external service was force-stopped: %v", err)
	}
	_ = connection.Close()
}

func TestCloseConfirmationMayRespondSynchronously(t *testing.T) {
	host := &testServiceHost{respondToClose: true}
	service := &Service{coordinator: &Coordinator{snapshot: defaultSnapshot()}, host: host}
	host.service = service
	completed := make(chan error, 1)
	go func() { completed <- service.Close() }()
	select {
	case err := <-completed:
		if err != nil || host.closeResponse != nil {
			t.Fatalf("close errors = %v, %v", err, host.closeResponse)
		}
	case <-time.After(time.Second):
		t.Fatal("Close() deadlocked while the renderer answered synchronously")
	}
	if err := service.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if host.exitConfirmEmits != 2 {
		t.Fatalf("exit confirmation emits = %d, want 2", host.exitConfirmEmits)
	}
}

func TestCloseConfirmationPendingStateSurvivesMissedEvent(t *testing.T) {
	host := &testServiceHost{}
	service := &Service{coordinator: &Coordinator{snapshot: defaultSnapshot()}, host: host}
	if err := service.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !service.HasPendingCloseConfirm() {
		t.Fatal("close confirmation was not left pending after the event was emitted")
	}
	if err := service.CloseConfirmResponse(LauncherCloseConfirmResponse{Action: "cancel"}); err != nil {
		t.Fatalf("CloseConfirmResponse() error = %v", err)
	}
	if service.HasPendingCloseConfirm() {
		t.Fatal("close confirmation remained pending after cancellation")
	}
}

func TestServiceShutdownCancelsExternalStopConfirmation(t *testing.T) {
	host := &testServiceHost{}
	service := &Service{
		coordinator: &Coordinator{process: NewProcessController(""), snapshot: defaultSnapshot()},
		host:        host,
	}
	if err := service.ServiceShutdown(); err != nil {
		t.Fatalf("ServiceShutdown() error = %v", err)
	}
	if len(host.externalStopResponses) != 1 || host.externalStopResponses[0] {
		t.Fatalf("external stop responses = %#v, want [false]", host.externalStopResponses)
	}
}

func TestStopAndShutdownCancelStartupBeforeWaitingForOperationLock(t *testing.T) {
	operations := []struct {
		name string
		run  func(*Coordinator)
	}{
		{name: "stop", run: func(coordinator *Coordinator) { _ = coordinator.Stop() }},
		{name: "shutdown", run: func(coordinator *Coordinator) { coordinator.Shutdown() }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			coordinator := &Coordinator{process: NewProcessController(""), snapshot: defaultSnapshot()}
			startupContext, finishStartup, allowed := coordinator.beginStartup()
			if !allowed {
				t.Fatal("initial startup was unexpectedly blocked")
			}
			defer finishStartup()
			coordinator.operationMu.Lock()

			completed := make(chan struct{})
			go func() {
				operation.run(coordinator)
				close(completed)
			}()

			select {
			case <-startupContext.Done():
			case <-time.After(time.Second):
				coordinator.operationMu.Unlock()
				t.Fatal("startup was not cancelled before waiting for the operation lock")
			}
			if _, finishLateStartup, lateAllowed := coordinator.beginStartup(); lateAllowed {
				finishLateStartup()
				coordinator.operationMu.Unlock()
				t.Fatal("a startup registered after stop or shutdown began")
			}
			coordinator.operationMu.Unlock()
			select {
			case <-completed:
			case <-time.After(time.Second):
				t.Fatal("operation did not complete after the startup lock was released")
			}
			_, finishLaterStartup, laterAllowed := coordinator.beginStartup()
			if operation.name == "stop" && !laterAllowed {
				t.Fatal("ordinary stop left future startups blocked")
			}
			if operation.name == "shutdown" && laterAllowed {
				finishLaterStartup()
				t.Fatal("shutdown allowed a future startup")
			}
			if laterAllowed {
				finishLaterStartup()
			}
		})
	}
}

func TestStartupGateStaysBlockedUntilEveryStopCompletes(t *testing.T) {
	coordinator := &Coordinator{}
	unblockFirst := coordinator.blockStartups(false)
	unblockSecond := coordinator.blockStartups(false)
	unblockFirst()
	if _, finishStartup, allowed := coordinator.beginStartup(); allowed {
		finishStartup()
		t.Fatal("startup gate reopened while another stop was still active")
	}
	unblockSecond()
	if _, finishStartup, allowed := coordinator.beginStartup(); !allowed {
		t.Fatal("startup gate remained blocked after every stop completed")
	} else {
		finishStartup()
	}
}

func TestStartReportsWhenStartupGateRejectsOperation(t *testing.T) {
	coordinator := &Coordinator{}
	unblock := coordinator.blockStartups(false)
	defer unblock()

	if err := coordinator.Start(); !errors.Is(err, errStartupBlocked) {
		t.Fatalf("Start() error = %v, want errStartupBlocked", err)
	}
}

func TestSaveSettingsCancelsStartupBeforeWaitingForOperationLock(t *testing.T) {
	root := createDevelopmentInstall(t)
	coordinator := NewCoordinator(root, "", 0, nil)
	coordinator.mu.Lock()
	coordinator.initialized = true
	coordinator.settings = LauncherSettings{InstallationRoot: root, CloseBehavior: closeAsk}
	coordinator.mu.Unlock()

	startupContext, finishStartup, allowed := coordinator.beginStartup()
	if !allowed {
		t.Fatal("startup was unexpectedly blocked")
	}
	defer finishStartup()
	coordinator.operationMu.Lock()

	result := make(chan error, 1)
	go func() {
		result <- coordinator.SaveSettings(LauncherSettings{InstallationRoot: root, CloseBehavior: closeTray})
	}()

	select {
	case <-startupContext.Done():
	case <-time.After(time.Second):
		coordinator.operationMu.Unlock()
		t.Fatal("SaveSettings() did not cancel startup before waiting for the operation lock")
	}
	coordinator.operationMu.Unlock()

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("SaveSettings() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("SaveSettings() remained blocked after startup released the operation lock")
	}
}

func TestStartupTimeoutKeepsRunningProcessVisibleWhenTerminationFails(t *testing.T) {
	coordinator := &Coordinator{process: NewProcessController(""), snapshot: defaultSnapshot()}
	killErr := errors.New("access denied")
	err := coordinator.finishStartupTimeout(operationContext{}, EnvironmentInspection{}, killErr, true)
	if !errors.Is(err, killErr) {
		t.Fatalf("finishStartupTimeout() error = %v, want wrapped kill error", err)
	}
	snapshot := coordinator.Snapshot()
	if snapshot.Launcher.ProcessLifecycle != "running" || snapshot.Launcher.ProcessOwnership != "launcher_managed" {
		t.Fatalf("timeout snapshot = %#v, want managed process to remain visible", snapshot.Launcher)
	}
}

func TestFailedUpdateReportsUnavailableServiceRecovery(t *testing.T) {
	root := t.TempDir()
	coordinator := NewCoordinator(root, "", 0, nil)
	coordinator.settings = LauncherSettings{InstallationRoot: root, CloseBehavior: closeAsk}
	coordinator.initialized = true
	installErr := errors.New("update helper unavailable")

	err := coordinator.recoverServiceAfterUpdateFailure(installErr)
	if !errors.Is(err, installErr) {
		t.Fatalf("recoverServiceAfterUpdateFailure() error = %v, want wrapped install error", err)
	}
	if !strings.Contains(err.Error(), "更新助手启动失败，且原服务恢复失败") {
		t.Fatalf("recoverServiceAfterUpdateFailure() error = %q, want combined recovery failure", err)
	}
}
