package lifecycle

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/internal/testenv"
)

func TestInstallServiceInstallsLocalDirectoryAndRefreshesCatalog(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "weather-src"), "weather")
	repository := &stubInstallRepository{}
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, repository, installerDeps{})
	defer service.Close()

	taskID, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusSucceeded)
	}
	if snapshot.Progress != 100 {
		t.Fatalf("unexpected progress: got %d want 100", snapshot.Progress)
	}
	if snapshot.Result == nil || snapshot.Result.Summary == "" {
		t.Fatalf("expected task result summary, got %#v", snapshot.Result)
	}

	if _, err := os.Stat(filepath.Join(installedRoot, "weather", "info.json")); err != nil {
		t.Fatalf("expected installed manifest to exist: %v", err)
	}

	installed, ok := catalog.Get("weather")
	if !ok {
		t.Fatal("expected installed plugin in refreshed catalog")
	}
	if installed.RegistrationState != "installed" {
		t.Fatalf("unexpected registration_state: got %q want installed", installed.RegistrationState)
	}
	if installed.DesiredState != "disabled" {
		t.Fatalf("unexpected desired_state: got %q want disabled", installed.DesiredState)
	}
	if installed.RuntimeState != "stopped" {
		t.Fatalf("unexpected runtime_state: got %q want stopped", installed.RuntimeState)
	}
	if repository.lastPackage.PluginID != "weather" {
		t.Fatalf("expected package metadata for weather, got %#v", repository.lastPackage)
	}
	if repository.lastPackage.SourceType != "local_directory" {
		t.Fatalf("unexpected source_type metadata: got %q want local_directory", repository.lastPackage.SourceType)
	}
	if repository.lastPackage.Version != "0.1.0" {
		t.Fatalf("unexpected version metadata: got %q want 0.1.0", repository.lastPackage.Version)
	}
	if repository.lastPackage.ManifestHash == "" || repository.lastPackage.PackageHash == "" {
		t.Fatalf("expected package metadata hashes to be populated, got %#v", repository.lastPackage)
	}
}

func TestInstallServiceInvokesAfterSuccessCallback(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "callback-src"), "callback-weather")
	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	defer service.Close()

	called := make(chan string, 1)
	service.SetAfterSuccess(func(ctx context.Context, pluginID string) error {
		if ctx == nil {
			t.Fatal("expected install callback context")
		}
		called <- pluginID
		return nil
	})

	taskID, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusSucceeded)
	}

	select {
	case pluginID := <-called:
		if pluginID != "callback-weather" {
			t.Fatalf("unexpected callback plugin id: got %q want callback-weather", pluginID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for install after-success callback")
	}
}

func TestInstallServiceFailsWhenAfterSuccessCallbackFails(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "callback-fail-src"), "callback-fail-weather")
	repository := &stubInstallRepository{}
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, repository, installerDeps{})
	defer service.Close()

	service.SetAfterSuccess(func(ctx context.Context, pluginID string) error {
		if ctx == nil {
			t.Fatal("expected install callback context")
		}
		if pluginID != "callback-fail-weather" {
			t.Fatalf("unexpected callback plugin id: got %q want callback-fail-weather", pluginID)
		}
		return fmt.Errorf("sync plugin render template callback-fail-weather: source conflict")
	})

	taskID, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusFailed {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusFailed)
	}
	if snapshot.Error == nil || snapshot.Error.Code != codePluginInstallFailed {
		t.Fatalf("unexpected task error: %#v", snapshot.Error)
	}
	if _, ok := catalog.Get("callback-fail-weather"); ok {
		t.Fatal("plugin remained in catalog after after-success failure")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "plugins", "installed", "callback-fail-weather")); !os.IsNotExist(err) {
		t.Fatalf("installed directory was not rolled back: %v", err)
	}
	if repository.deletedPackage != "callback-fail-weather" {
		t.Fatalf("package metadata was not rolled back: got %q", repository.deletedPackage)
	}
}

func TestInstallServiceInstallsLocalZip(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "zip-src"), "zip-weather")
	archivePath := filepath.Join(t.TempDir(), "zip-weather.zip")
	writePluginZip(t, archivePath, sourceDir)

	service, catalog := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	defer service.Close()

	taskID, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "local_zip",
		Source:     archivePath,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusSucceeded)
	}

	if _, ok := catalog.Get("zip-weather"); !ok {
		t.Fatal("expected zip-installed plugin in refreshed catalog")
	}
}

func TestInstallServiceMapsRemoteDownloadLimitToStableError(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	service, _ := newInstallTestService(t, t.TempDir(), registry, nil, &stubInstallRepository{}, installerDeps{
		downloadFile: func(context.Context, string, string) error {
			return fmt.Errorf("%w: fixture", errPluginPackageResourceLimit)
		},
	})
	defer service.Close()

	_, err := service.Inspect(context.Background(), plugins.InstallRequest{
		SourceType: "remote_url",
		Source:     "https://downloads.example/plugin.zip",
	})
	if InstallErrorCode(err) != codePackageResourceLimit {
		t.Fatalf("unexpected inspection error: %v", err)
	}
	if len(registry.List()) != 0 {
		t.Fatal("failed inspection created an install task")
	}
}

func TestInstallServiceBindsAcceptanceToInspectionDigestAndTrust(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "inspect-src"), "inspect-weather")
	service, _ := newInstallTestService(t, t.TempDir(), registry, nil, &stubInstallRepository{}, installerDeps{})
	defer service.Close()

	request := plugins.InstallRequest{SourceType: "local_directory", Source: sourceDir}
	inspection, err := service.Inspect(context.Background(), request)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	request.InspectionID = inspection.InspectionID
	request.PackageSHA256 = inspection.PackageSHA256
	if _, err := service.Accept(context.Background(), request); !errors.Is(err, plugins.ErrTrustedCodeConfirmation) {
		t.Fatalf("untrusted acceptance error = %v", err)
	}

	request.TrustedCodeConfirmed = true
	request.PackageSHA256 = strings.Repeat("f", 64)
	if _, err := service.Accept(context.Background(), request); !errors.Is(err, plugins.ErrInstallDigestMismatch) {
		t.Fatalf("digest mismatch error = %v", err)
	}
	if len(registry.List()) != 0 {
		t.Fatal("rejected inspection created a task")
	}

	request.PackageSHA256 = inspection.PackageSHA256
	if _, err := service.Accept(context.Background(), request); err != nil {
		t.Fatalf("accept inspected package: %v", err)
	}
}

func TestInstallServiceRejectsFullQueueBeforeTaskCreation(t *testing.T) {
	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "queue-src"), "queue-weather")
	started := make(chan struct{})
	release := make(chan struct{})
	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	service.SetAfterSuccess(func(ctx context.Context, _ string) error {
		select {
		case <-started:
		default:
			close(started)
		}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	request := plugins.InstallRequest{SourceType: "local_directory", Source: sourceDir}
	if _, err := acceptInspected(t, service, request); err != nil {
		t.Fatalf("submit running install: %v", err)
	}
	<-started
	for index := 0; index < 32; index++ {
		if _, err := acceptInspected(t, service, request); err != nil {
			t.Fatalf("submit queued install %d: %v", index, err)
		}
	}
	before := len(registry.List())
	if _, err := acceptInspected(t, service, request); !errors.Is(err, tasks.ErrQueueFull) {
		t.Fatalf("queue-full error = %v, want tasks.ErrQueueFull", err)
	}
	if after := len(registry.List()); after != before {
		t.Fatalf("queue-full install created a task: before=%d after=%d", before, after)
	}
	close(release)
	if err := service.Close(); err != nil {
		t.Fatalf("close install service: %v", err)
	}
}

func TestExtractZipSourceRejectsUnsafeEntriesAndCompressionBombs(t *testing.T) {
	tests := []struct {
		name    string
		header  zip.FileHeader
		content string
		code    string
	}{
		{
			name: "symlink",
			header: func() zip.FileHeader {
				header := zip.FileHeader{Name: "plugin/link", Method: zip.Store}
				header.SetMode(os.ModeSymlink | 0o777)
				return header
			}(),
			content: "../outside",
			code:    codePackageUnsafeEntry,
		},
		{
			name:    "zip bomb ratio",
			header:  zip.FileHeader{Name: "plugin/payload.txt", Method: zip.Deflate},
			content: strings.Repeat("0", 1024*1024),
			code:    codePackageResourceLimit,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			archivePath := filepath.Join(t.TempDir(), "unsafe.zip")
			writeZipEntries(t, archivePath, []zipTestEntry{{header: test.header, content: test.content}})
			_, err := extractZipSource(context.Background(), archivePath, t.TempDir())
			if err == nil {
				t.Fatal("expected unsafe archive to be rejected")
			}
			var installErr *installTaskError
			if !errors.As(err, &installErr) || installErr.Code != test.code {
				t.Fatalf("archive error = %#v, want code %s", err, test.code)
			}
		})
	}
}

func TestExtractZipSourceRejectsEntryCountLimit(t *testing.T) {
	entries := make([]zipTestEntry, maxPluginArchiveEntries+1)
	for index := range entries {
		entries[index].header = zip.FileHeader{
			Name:   fmt.Sprintf("plugin/file-%05d.txt", index),
			Method: zip.Store,
		}
	}
	archivePath := filepath.Join(t.TempDir(), "too-many-entries.zip")
	writeZipEntries(t, archivePath, entries)

	_, err := extractZipSource(context.Background(), archivePath, t.TempDir())
	var installErr *installTaskError
	if !errors.As(err, &installErr) || installErr.Code != codePackageResourceLimit {
		t.Fatalf("entry-limit error = %#v, want %s", err, codePackageResourceLimit)
	}
}

func TestValidatePluginDownloadRedirect(t *testing.T) {
	request := func(rawURL string) *http.Request {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			t.Fatalf("parse URL: %v", err)
		}
		return &http.Request{URL: parsed}
	}

	if err := validatePluginDownloadRedirect(request("https://downloads.example/plugin.zip"), make([]*http.Request, maxPluginDownloadRedirects)); err != nil {
		t.Fatalf("valid redirect rejected: %v", err)
	}
	for _, rawURL := range []string{
		"http://downloads.example/plugin.zip",
		"https://user:password@downloads.example/plugin.zip",
	} {
		if err := validatePluginDownloadRedirect(request(rawURL), nil); err == nil {
			t.Fatalf("unsafe redirect accepted: %s", rawURL)
		}
	}
	if err := validatePluginDownloadRedirect(request("https://downloads.example/plugin.zip"), make([]*http.Request, maxPluginDownloadRedirects+1)); err == nil {
		t.Fatal("redirect limit was not enforced")
	}
}

func TestInstallServiceRejectsInvalidRenderTemplatePackage(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "template-src"), "template-weather")
	addRenderTemplateDeclarationToManifest(t, sourceDir, "templates/card")

	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	service.SetRenderTemplateValidator(func(snapshot plugins.Snapshot) error {
		return validateInstallRenderTemplates(snapshot)
	})
	defer service.Close()

	taskID, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusFailed {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusFailed)
	}
	if snapshot.Error == nil || snapshot.Error.Code != codePluginInstallFailed {
		t.Fatalf("unexpected task error: %#v", snapshot.Error)
	}
}

func TestInstallServiceInstallsRenderTemplatePackage(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "template-ok-src"), "template-ok-weather")
	addRenderTemplateDeclarationToManifest(t, sourceDir, "templates/card")
	writeInstallRenderTemplate(t, filepath.Join(sourceDir, "templates", "card"), "card")

	service, catalog := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	service.SetRenderTemplateValidator(validateInstallRenderTemplates)
	defer service.Close()

	taskID, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("unexpected task status: got %q want %q (%#v)", snapshot.Status, tasks.StatusSucceeded, snapshot.Error)
	}
	installed, ok := catalog.Get("template-ok-weather")
	if !ok {
		t.Fatal("expected installed plugin in refreshed catalog")
	}
	if len(installed.RenderTemplates) != 1 || installed.RenderTemplates[0].Path != "templates/card" {
		t.Fatalf("unexpected render_templates: %#v", installed.RenderTemplates)
	}
}

func TestInstallServiceRejectsInvalidRenderTemplateManifest(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "template-bad-src"), "template-bad-weather")
	addRenderTemplateDeclarationToManifest(t, sourceDir, "templates/card")
	writeInstallRenderTemplate(t, filepath.Join(sourceDir, "templates", "card"), "card/escaped")

	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	service.SetRenderTemplateValidator(validateInstallRenderTemplates)
	defer service.Close()

	taskID, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusFailed {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusFailed)
	}
	if snapshot.Error == nil || snapshot.Error.Code != codePluginInstallFailed {
		t.Fatalf("unexpected task error: %#v", snapshot.Error)
	}
}

func TestInstallServiceFailsDuplicatePluginID(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	existing := []plugins.Snapshot{{
		PluginID:          "hello-go",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
		DisplayState:      "discovered",
	}}
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "dup-src"), "hello-go")
	service, _ := newInstallTestService(t, repoRoot, registry, existing, &stubInstallRepository{}, installerDeps{})
	defer service.Close()

	taskID, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusFailed {
		t.Fatalf("unexpected task status: got %q want %q", snapshot.Status, tasks.StatusFailed)
	}
	if snapshot.Error == nil || snapshot.Error.Code != codePluginInstallFailed {
		t.Fatalf("unexpected task error: %#v", snapshot.Error)
	}
}

func TestInstallServiceCancelsRunningTask(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "cancel-src"), "cancel-weather")

	installStarted := make(chan struct{}, 1)
	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	service.SetAfterSuccess(func(ctx context.Context, _ string) error {
		select {
		case installStarted <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	})
	defer service.Close()

	taskID, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "local_directory",
		Source:     sourceDir,
	})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	select {
	case <-installStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for running install phase")
	}

	if !service.Cancel(taskID) {
		t.Fatal("expected running install cancellation to be accepted")
	}

	snapshot := waitForTaskCompletion(t, registry, taskID)
	if snapshot.Status != tasks.StatusCancelled {
		t.Fatalf("unexpected cancelled status: got %q want %q", snapshot.Status, tasks.StatusCancelled)
	}
}

func TestInstallServiceRejectsLegacyRuntimeManifest(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "legacy-src"), "legacy-python")
	infoPath := filepath.Join(sourceDir, "info.json")
	content, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["runtime"] = "python"
	manifest["manifest_version"] = "1"
	manifest["entry"] = "main.py"
	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(infoPath, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}

	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	defer service.Close()
	_, err = service.Inspect(context.Background(), plugins.InstallRequest{SourceType: "local_directory", Source: sourceDir})
	if InstallErrorCode(err) != codePluginArtifactInvalid {
		t.Fatalf("Inspect() error = %v, want %s", err, codePluginArtifactInvalid)
	}
}

func newInstallTestService(t *testing.T, repoRoot string, registry *tasks.Registry, initial []plugins.Snapshot, repository plugins.DesiredStateRepository, deps installerDeps) (*InstallService, *testCatalog) {
	t.Helper()

	validator, err := config.Compile(filepath.Join("..", "..", "..", "..", "contracts", "plugin-info.schema.json"))
	if err != nil {
		t.Fatalf("compile plugin-info schema: %v", err)
	}

	examplesRoot := filepath.Join(repoRoot, "examples", "plugins")
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	if err := os.MkdirAll(examplesRoot, 0o755); err != nil {
		t.Fatalf("create examples root: %v", err)
	}
	if err := os.MkdirAll(installedRoot, 0o755); err != nil {
		t.Fatalf("create installed root: %v", err)
	}

	catalog := newTestCatalog(initial)
	service, err := newInstallService(
		nil,
		registry,
		catalog,
		repository,
		validator,
		repoRoot,
		[]plugincatalog.ScanRoot{
			{Label: "examples/plugins", Path: examplesRoot},
			{Label: "plugins/installed", Path: installedRoot},
		},
		installServiceTimeout(),
		deps,
	)
	if err != nil {
		t.Fatalf("newInstallService failed: %v", err)
	}
	return service, catalog
}

func installServiceTimeout() time.Duration {
	if testing.CoverMode() != "" || testenv.RaceEnabled {
		return 20 * time.Second
	}
	return 15 * time.Second
}

type stubInstallRepository struct {
	saved          map[string]string
	lastPackage    plugins.PackageMetadata
	deletedPackage string
}

func acceptInspected(t *testing.T, service *InstallService, request plugins.InstallRequest) (string, error) {
	t.Helper()
	if request.SourceType == "local_directory" {
		refreshInstallArtifact(t, request.Source)
	}
	inspection, err := service.Inspect(context.Background(), request)
	if err != nil {
		return "", err
	}
	request.InspectionID = inspection.InspectionID
	request.PackageSHA256 = inspection.PackageSHA256
	request.TrustedCodeConfirmed = true
	return service.Accept(context.Background(), request)
}

func (r *stubInstallRepository) LoadDesiredStates(context.Context) (map[string]string, error) {
	if r == nil {
		return nil, nil
	}
	return r.saved, nil
}

func (r *stubInstallRepository) SaveDesiredState(_ context.Context, pluginID string, desiredState string, _ time.Time) error {
	if r.saved == nil {
		r.saved = make(map[string]string)
	}
	r.saved[pluginID] = desiredState
	return nil
}

func (r *stubInstallRepository) SavePackageMetadata(_ context.Context, pkg plugins.PackageMetadata) error {
	r.lastPackage = pkg
	return nil
}

func (r *stubInstallRepository) DeleteDesiredState(_ context.Context, _ string) error {
	return nil
}

func (r *stubInstallRepository) DeletePackageMetadata(_ context.Context, pluginID string) error {
	r.deletedPackage = pluginID
	return nil
}

func writeInstallSourcePlugin(t *testing.T, root, pluginID string) string {
	t.Helper()

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("create plugin root: %v", err)
	}

	targetPlatform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	entry := "bin/" + pluginID
	manifest := map[string]any{
		"id":                      pluginID,
		"name":                    pluginID,
		"version":                 "0.1.0",
		"manifest_version":        "2",
		"plugin_protocol_version": "1",
		"runtime":                 "go",
		"entry":                   entry,
		"platforms":               []string{"windows-x64", "linux-x64", "macos-arm64"},
		"license":                 "MIT",
		"description":             "test plugin",
		"author":                  "raylea",
		"capabilities":            []string{"event.subscribe"},
	}

	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "info.json"), append(manifestBytes, '\n'), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	backendRelative := entry
	if targetPlatform == "windows-x64" {
		backendRelative += ".exe"
	}
	backendPath := filepath.Join(root, filepath.FromSlash(backendRelative))
	if err := os.MkdirAll(filepath.Dir(backendPath), 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyInstallTestFile(t, executable, backendPath)
	refreshInstallArtifact(t, root)
	return root
}

func refreshInstallArtifact(t *testing.T, root string) {
	t.Helper()
	manifestBytes, err := os.ReadFile(filepath.Join(root, "info.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	pluginID, _ := manifest["id"].(string)
	pluginVersion, _ := manifest["version"].(string)
	logicalEntry, _ := manifest["entry"].(string)
	targetPlatform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	backendRelative := logicalEntry
	if targetPlatform == "windows-x64" {
		backendRelative += ".exe"
	}
	files := make([]map[string]any, 0)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "artifact.json" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		digest := sha256.Sum256(content)
		role := "data"
		switch {
		case relative == "info.json":
			role = "manifest"
		case relative == filepath.ToSlash(backendRelative):
			role = "backend"
		case strings.HasPrefix(relative, "templates/"):
			role = "render_template"
		case strings.HasPrefix(relative, "ui/") || strings.HasPrefix(relative, "web/"):
			role = "ui"
		}
		files = append(files, map[string]any{"path": relative, "role": role, "size": info.Size(), "sha256": hex.EncodeToString(digest[:])})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	document := map[string]any{
		"artifact_version": "1", "plugin_id": pluginID, "plugin_version": pluginVersion, "target_platform": targetPlatform,
		"manifest_sha256": hex.EncodeToString(manifestDigest[:]), "files": files,
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifact.json"), append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func copyInstallTestFile(t *testing.T, source, destination string) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func addRenderTemplateDeclarationToManifest(t *testing.T, pluginRoot, templatePath string) {
	t.Helper()

	infoPath := filepath.Join(pluginRoot, "info.json")
	bytes, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	manifest["render_templates"] = []map[string]any{{"path": templatePath}}
	manifest["capabilities"] = []string{"event.subscribe", "render.image"}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("encode manifest: %v", err)
	}
	if err := os.WriteFile(infoPath, encoded, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func writeInstallRenderTemplate(t *testing.T, templateDir, templateID string) {
	t.Helper()

	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create template dir: %v", err)
	}
	html := "<html><body>{{ .title }}</body></html>"
	files := map[string]string{
		"template.json": fmt.Sprintf(`{
  "id": %q,
  "version": "1",
  "entry_html": "template.html",
  "stylesheet": "styles.css",
  "input_schema": "input.schema.json",
  "width": 320,
  "height": 240
}`, templateID),
		"template.html":     html,
		"styles.css":        "body { margin: 0; }",
		"input.schema.json": `{"type":"object","additionalProperties":true}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(templateDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write template %s: %v", name, err)
		}
	}
}

func validateInstallRenderTemplates(snapshot plugins.Snapshot) error {
	for _, declared := range snapshot.RenderTemplates {
		templateDir := filepath.Join(snapshot.PackageRootPath, filepath.FromSlash(declared.Path))
		if info, err := os.Stat(templateDir); err != nil || !info.IsDir() {
			return fmt.Errorf("load plugin render template %s: template directory is missing", snapshot.PluginID)
		}
		manifestPath := filepath.Join(templateDir, "template.json")
		document, err := config.LoadJSONFile(manifestPath)
		if err != nil {
			return fmt.Errorf("load plugin render template %s: %w", snapshot.PluginID, err)
		}
		manifest, ok := document.(map[string]any)
		if !ok {
			return fmt.Errorf("load plugin render template %s: manifest must be an object", snapshot.PluginID)
		}
		id, ok := manifest["id"].(string)
		if !ok || id == "" || strings.Contains(id, "/") || strings.Contains(id, "\\") {
			return fmt.Errorf("load plugin render template %s: template id is invalid", snapshot.PluginID)
		}
	}
	return nil
}

func writePluginZip(t *testing.T, archivePath, sourceDir string) {
	t.Helper()

	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	defer file.Close()

	writer := zip.NewWriter(file)

	if err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(filepath.Dir(sourceDir), path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if info.IsDir() {
			_, err := writer.Create(name + "/")
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = zip.Deflate
		entryWriter, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = entryWriter.Write(bytes)
		return err
	}); err != nil {
		t.Fatalf("write zip contents: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
}

type zipTestEntry struct {
	header  zip.FileHeader
	content string
}

func writeZipEntries(t *testing.T, archivePath string, entries []zipTestEntry) {
	t.Helper()
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	for _, entry := range entries {
		entryWriter, err := writer.CreateHeader(&entry.header)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := entryWriter.Write([]byte(entry.content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}

func waitForTaskCompletion(t *testing.T, registry *tasks.Registry, taskID string) tasks.Snapshot {
	t.Helper()

	deadline := time.Now().Add(taskCompletionTimeout())
	for time.Now().Before(deadline) {
		snapshot, ok := registry.Get(taskID)
		if ok {
			switch snapshot.Status {
			case tasks.StatusSucceeded, tasks.StatusFailed, tasks.StatusCancelled, tasks.StatusInterrupted:
				return snapshot
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for task %s to complete", taskID)
	return tasks.Snapshot{}
}

func taskCompletionTimeout() time.Duration {
	if testing.CoverMode() != "" || testenv.RaceEnabled {
		return 20 * time.Second
	}
	return 15 * time.Second
}
