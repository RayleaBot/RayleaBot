package lifecycle

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/internal/testenv"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestInstallServiceInstallsLocalDirectoryAndRefreshesCatalog(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	installedRoot := filepath.Join(repoRoot, "plugins", "installed")
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "weather-src"), "weather")
	repository := &stubInstallRepository{}
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, repository, installerDeps{})
	defer func(release func() error) { _ = release() }(service.Close)

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
	if repository.lastPackage.PackageHash == "" {
		t.Fatalf("expected package metadata hashes to be populated, got %#v", repository.lastPackage)
	}
}

func TestInstallServiceInvokesAfterSuccessCallback(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "callback-src"), "callback-weather")
	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	defer func(release func() error) { _ = release() }(service.Close)

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
	defer func(release func() error) { _ = release() }(service.Close)

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

func TestInstallServiceAtomicallyReplacesInstalledPlugin(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	repository := &stubInstallRepository{}
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, repository, installerDeps{})
	defer func(release func() error) { _ = release() }(service.Close)

	initial := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "replace-initial"), "replace-weather")
	initialTask, err := acceptInspected(t, service, plugins.InstallRequest{SourceType: "local_directory", Source: initial})
	if err != nil {
		t.Fatalf("install initial plugin: %v", err)
	}
	if snapshot := waitForTaskCompletion(t, registry, initialTask); snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("initial task status = %q", snapshot.Status)
	}

	replacement := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "replace-next"), "replace-weather")
	setInstallSourcePluginVersion(t, replacement, "0.2.0")
	stopped := make(chan string, 1)
	service.SetBeforeReplace(func(_ context.Context, pluginID string) error { stopped <- pluginID; return nil })
	replaceTask, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "development", Source: replacement, ResolvedSourceType: "local_directory",
		ResolvedSource: replacement, ReplaceExisting: true,
	})
	if err != nil {
		t.Fatalf("replace plugin: %v", err)
	}
	if snapshot := waitForTaskCompletion(t, registry, replaceTask); snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("replacement task = %#v", snapshot)
	}
	if pluginID := <-stopped; pluginID != "replace-weather" {
		t.Fatalf("stopped plugin = %q", pluginID)
	}

	installed, ok := catalog.Get("replace-weather")
	if !ok || installed.Version != "0.2.0" {
		t.Fatalf("installed snapshot = %#v, want version 0.2.0", installed)
	}
	metadata := repository.packages["replace-weather"]
	if metadata.Version != "0.2.0" || metadata.SourceType != "development" || metadata.SourceRef != replacement {
		t.Fatalf("replacement metadata = %#v", metadata)
	}
}

func TestInstallServiceRestoresLastGoodPluginAndMetadataWhenReplacementFinalizationFails(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	repository := &stubInstallRepository{}
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, repository, installerDeps{})
	defer func(release func() error) { _ = release() }(service.Close)

	initial := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "rollback-initial"), "rollback-weather")
	initialTask, err := acceptInspected(t, service, plugins.InstallRequest{SourceType: "local_directory", Source: initial})
	if err != nil {
		t.Fatalf("install initial plugin: %v", err)
	}
	if snapshot := waitForTaskCompletion(t, registry, initialTask); snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("initial task status = %q", snapshot.Status)
	}
	initialMetadata := repository.packages["rollback-weather"]

	replacement := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "rollback-next"), "rollback-weather")
	setInstallSourcePluginVersion(t, replacement, "0.2.0")
	rolledBack := make(chan string, 1)
	service.SetAfterRollback(func(_ context.Context, pluginID string) error { rolledBack <- pluginID; return nil })
	service.SetAfterSuccess(func(context.Context, string) error { return errors.New("template finalization failed") })
	replaceTask, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "development", Source: replacement, ResolvedSourceType: "local_directory",
		ResolvedSource: replacement, ReplaceExisting: true,
	})
	if err != nil {
		t.Fatalf("replace plugin: %v", err)
	}
	if snapshot := waitForTaskCompletion(t, registry, replaceTask); snapshot.Status != tasks.StatusFailed {
		t.Fatalf("replacement task = %#v, want failed", snapshot)
	}
	if pluginID := <-rolledBack; pluginID != "rollback-weather" {
		t.Fatalf("rolled back plugin = %q", pluginID)
	}

	installed, ok := catalog.Get("rollback-weather")
	if !ok || installed.Version != "0.1.0" {
		t.Fatalf("rolled-back snapshot = %#v, want version 0.1.0", installed)
	}
	if metadata := repository.packages["rollback-weather"]; metadata != initialMetadata {
		t.Fatalf("rolled-back metadata = %#v, want %#v", metadata, initialMetadata)
	}
	manifestPath := filepath.Join(repoRoot, "plugins", "installed", "rollback-weather", "info.json")
	if version := readInstallManifestVersion(t, manifestPath); version != "0.1.0" {
		t.Fatalf("rolled-back manifest version = %q", version)
	}
}

func TestInstallServiceResumesLastGoodPluginWhenReplacementRenameFails(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	repository := &stubInstallRepository{}
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, repository, installerDeps{})
	defer func(release func() error) { _ = release() }(service.Close)

	initial := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "rename-rollback-initial"), "rename-rollback-weather")
	initialTask, err := acceptInspected(t, service, plugins.InstallRequest{SourceType: "local_directory", Source: initial})
	if err != nil {
		t.Fatalf("install initial plugin: %v", err)
	}
	if snapshot := waitForTaskCompletion(t, registry, initialTask); snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("initial task status = %q", snapshot.Status)
	}

	stopped := make(chan string, 1)
	resumed := make(chan string, 1)
	service.SetBeforeReplace(func(_ context.Context, pluginID string) error { stopped <- pluginID; return nil })
	service.SetAfterRollback(func(_ context.Context, pluginID string) error { resumed <- pluginID; return nil })
	service.deps.waitRename = func(context.Context) error { return nil }
	service.deps.rename = func(source, target string) error {
		if filepath.Base(source) == "candidate" && filepath.Base(target) == "rename-rollback-weather" {
			return errors.New("injected candidate rename failure")
		}
		return os.Rename(source, target)
	}

	replacement := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "rename-rollback-next"), "rename-rollback-weather")
	setInstallSourcePluginVersion(t, replacement, "0.2.0")
	replaceTask, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "development", Source: replacement, ResolvedSourceType: "local_directory",
		ResolvedSource: replacement, ReplaceExisting: true,
	})
	if err != nil {
		t.Fatalf("replace plugin: %v", err)
	}
	if snapshot := waitForTaskCompletion(t, registry, replaceTask); snapshot.Status != tasks.StatusFailed {
		t.Fatalf("replacement task = %#v, want failed", snapshot)
	}
	if pluginID := <-stopped; pluginID != "rename-rollback-weather" {
		t.Fatalf("stopped plugin = %q", pluginID)
	}
	if pluginID := <-resumed; pluginID != "rename-rollback-weather" {
		t.Fatalf("resumed plugin = %q", pluginID)
	}

	installed, ok := catalog.Get("rename-rollback-weather")
	if !ok || installed.Version != "0.1.0" {
		t.Fatalf("catalog snapshot = %#v, want version 0.1.0", installed)
	}
	manifestPath := filepath.Join(repoRoot, "plugins", "installed", "rename-rollback-weather", "info.json")
	if version := readInstallManifestVersion(t, manifestPath); version != "0.1.0" {
		t.Fatalf("restored manifest version = %q", version)
	}
}

func TestInstallServiceRetriesTransientReplacementRename(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	repository := &stubInstallRepository{}
	service, catalog := newInstallTestService(t, repoRoot, registry, nil, repository, installerDeps{})
	defer func(release func() error) { _ = release() }(service.Close)

	initial := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "retry-initial"), "retry-weather")
	initialTask, err := acceptInspected(t, service, plugins.InstallRequest{SourceType: "local_directory", Source: initial})
	if err != nil {
		t.Fatalf("install initial plugin: %v", err)
	}
	if snapshot := waitForTaskCompletion(t, registry, initialTask); snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("initial task status = %q", snapshot.Status)
	}

	var activationAttempts atomic.Int32
	service.deps.retryRename = func(error) bool { return true }
	service.deps.waitRename = func(context.Context) error { return nil }
	service.deps.rename = func(source, target string) error {
		if filepath.Base(source) == "candidate" && filepath.Base(target) == "retry-weather" {
			if activationAttempts.Add(1) < 3 {
				return errors.New("injected transient candidate rename failure")
			}
		}
		return os.Rename(source, target)
	}

	replacement := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "retry-next"), "retry-weather")
	setInstallSourcePluginVersion(t, replacement, "0.2.0")
	replaceTask, err := acceptInspected(t, service, plugins.InstallRequest{
		SourceType: "development", Source: replacement, ResolvedSourceType: "local_directory",
		ResolvedSource: replacement, ReplaceExisting: true,
	})
	if err != nil {
		t.Fatalf("replace plugin: %v", err)
	}
	if snapshot := waitForTaskCompletion(t, registry, replaceTask); snapshot.Status != tasks.StatusSucceeded {
		t.Fatalf("replacement task = %#v", snapshot)
	}
	if attempts := activationAttempts.Load(); attempts != 3 {
		t.Fatalf("activation rename attempts = %d, want 3", attempts)
	}
	installed, ok := catalog.Get("retry-weather")
	if !ok || installed.Version != "0.2.0" {
		t.Fatalf("catalog snapshot = %#v, want version 0.2.0", installed)
	}
}

func TestInstallRenameDoesNotRetryPermanentErrors(t *testing.T) {
	var attempts atomic.Int32
	permanentErr := errors.New("permanent rename failure")
	service := &InstallService{deps: installerDeps{
		rename: func(string, string) error {
			attempts.Add(1)
			return permanentErr
		},
		retryRename: func(error) bool { return false },
		waitRename:  func(context.Context) error { t.Fatal("waited after a permanent error"); return nil },
	}}

	err := service.renameInstallPath(context.Background(), "source", "target")
	if !errors.Is(err, permanentErr) || attempts.Load() != 1 {
		t.Fatalf("rename result = %v after %d attempts", err, attempts.Load())
	}
}

func TestInstallRenamePreservesFailureWhenRetryIsCancelled(t *testing.T) {
	renameErr := errors.New("rename blocked")
	ctx, cancel := context.WithCancel(context.Background())
	service := &InstallService{deps: installerDeps{
		rename:      func(string, string) error { return renameErr },
		retryRename: func(error) bool { return true },
		waitRename: func(context.Context) error {
			cancel()
			return context.Canceled
		},
	}}

	err := service.renameInstallPath(ctx, "source", "target")
	if !errors.Is(err, renameErr) || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled rename error = %v", err)
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
	defer func(release func() error) { _ = release() }(service.Close)

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

func TestInstallServiceRejectsCatalogArchiveMismatch(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "catalog-integrity-src"), "catalog-integrity-weather")
	archivePath := filepath.Join(t.TempDir(), "catalog-integrity-weather.zip")
	writePluginZip(t, archivePath, sourceDir)
	for _, testCase := range []struct {
		name         string
		expectedHash string
	}{
		{name: "archive digest", expectedHash: strings.Repeat("f", 64)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			registry := tasks.NewRegistry()
			service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
			defer func(release func() error) { _ = release() }(service.Close)
			_, err := service.Inspect(context.Background(), plugins.InstallRequest{
				SourceType:            "catalog",
				Source:                "official/catalog-integrity-weather@0.1.0/windows-x64",
				ResolvedSourceType:    "local_zip",
				ResolvedSource:        archivePath,
				ExpectedArchiveSHA256: testCase.expectedHash,
			})
			if InstallErrorCode(err) != "plugin.store_integrity_mismatch" {
				t.Fatalf("Inspect() error = %v, want plugin.store_integrity_mismatch", err)
			}
		})
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
	defer func(release func() error) { _ = release() }(service.Close)

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
	defer func(release func() error) { _ = release() }(service.Close)

	request := plugins.InstallRequest{SourceType: "local_directory", Source: sourceDir, TrustedCodeRequired: true}
	inspection, err := service.Inspect(context.Background(), request)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	acceptance := plugins.InstallAcceptance{InspectionID: inspection.InspectionID, PackageSHA256: inspection.PackageSHA256}
	if _, err := service.Accept(context.Background(), acceptance); !errors.Is(err, plugins.ErrTrustedCodeConfirmation) {
		t.Fatalf("untrusted acceptance error = %v", err)
	}

	acceptance.TrustedCodeConfirmed = true
	acceptance.PackageSHA256 = strings.Repeat("f", 64)
	if _, err := service.Accept(context.Background(), acceptance); !errors.Is(err, plugins.ErrInstallDigestMismatch) {
		t.Fatalf("digest mismatch error = %v", err)
	}
	if len(registry.List()) != 0 {
		t.Fatal("rejected inspection created a task")
	}

	acceptance.PackageSHA256 = inspection.PackageSHA256
	if _, err := service.Accept(context.Background(), acceptance); err != nil {
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
	defer func(release func() error) { _ = release() }(service.Close)

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
	defer func(release func() error) { _ = release() }(service.Close)

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
	defer func(release func() error) { _ = release() }(service.Close)

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
	defer func(release func() error) { _ = release() }(service.Close)

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
	defer func(release func() error) { _ = release() }(service.Close)

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
	defer func(release func() error) { _ = release() }(service.Close)
	_, err = service.Inspect(context.Background(), plugins.InstallRequest{SourceType: "local_directory", Source: sourceDir})
	if InstallErrorCode(err) != "plugin.contract_unsupported" {
		t.Fatalf("Inspect() error = %v, want plugin.contract_unsupported", err)
	}
}

func TestInstallServiceRejectsIncompatibleMinimumCoreVersion(t *testing.T) {
	t.Parallel()

	registry := tasks.NewRegistry()
	repoRoot := t.TempDir()
	sourceDir := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "future-core-src"), "future-core")
	infoPath := filepath.Join(sourceDir, "info.json")
	payload, err := os.ReadFile(infoPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["min_core_version"] = "999.0.0"
	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(infoPath, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	refreshInstallArtifact(t, sourceDir)

	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	defer func(release func() error) { _ = release() }(service.Close)
	_, err = service.Inspect(context.Background(), plugins.InstallRequest{SourceType: "local_directory", Source: sourceDir})
	if InstallErrorCode(err) != "plugin.core_version_incompatible" {
		t.Fatalf("Inspect() error = %v, want plugin.core_version_incompatible", err)
	}
}

func newInstallTestService(t *testing.T, repoRoot string, registry *tasks.Registry, initial []plugins.Snapshot, repository plugins.DesiredStateRepository, deps installerDeps) (*InstallService, *testCatalog) {
	t.Helper()
	testutil.WriteBuildInfo(t, repoRoot, "0.4.0")

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

func TestInstallRejectsUnknownCoreVersion(t *testing.T) {
	repoRoot := t.TempDir()
	registry := tasks.NewRegistry()
	service, _ := newInstallTestService(t, repoRoot, registry, nil, &stubInstallRepository{}, installerDeps{})
	t.Cleanup(func() { _ = service.Close() })
	if err := os.Remove(filepath.Join(repoRoot, "build_info.json")); err != nil {
		t.Fatal(err)
	}
	source := writeInstallSourcePlugin(t, filepath.Join(t.TempDir(), "weather"), "weather")
	_, err := service.Inspect(t.Context(), plugins.InstallRequest{SourceType: "local_directory", Source: source})
	if InstallErrorCode(err) != "plugin.core_version_incompatible" {
		t.Fatalf("unknown build accepted an installation: %v", err)
	}
}

func installServiceTimeout() time.Duration {
	if testing.CoverMode() != "" || testenv.RaceEnabled {
		return 20 * time.Second
	}
	return 15 * time.Second
}

type stubInstallRepository struct {
	saved          map[string]string
	packages       map[string]plugins.PackageMetadata
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
	return service.Accept(context.Background(), plugins.InstallAcceptance{
		InspectionID:         inspection.InspectionID,
		PackageSHA256:        inspection.PackageSHA256,
		TrustedCodeConfirmed: true,
	})
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
	if r.packages == nil {
		r.packages = make(map[string]plugins.PackageMetadata)
	}
	r.packages[pkg.PluginID] = pkg
	r.lastPackage = pkg
	return nil
}

func (r *stubInstallRepository) LoadAllPackageMetadata(context.Context) (map[string]plugins.PackageMetadata, error) {
	result := make(map[string]plugins.PackageMetadata, len(r.packages))
	for pluginID, metadata := range r.packages {
		result[pluginID] = metadata
	}
	return result, nil
}

func (r *stubInstallRepository) DeleteDesiredState(_ context.Context, _ string) error {
	return nil
}

func (r *stubInstallRepository) DeletePackageMetadata(_ context.Context, pluginID string) error {
	r.deletedPackage = pluginID
	delete(r.packages, pluginID)
	return nil
}

func setInstallSourcePluginVersion(t *testing.T, root, version string) {
	t.Helper()
	manifestPath := filepath.Join(root, "info.json")
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["version"] = version
	payload, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, append(payload, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	refreshInstallArtifact(t, root)
}

func readInstallManifestVersion(t *testing.T, manifestPath string) string {
	t.Helper()
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest.Version
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
		"id":               pluginID,
		"name":             pluginID,
		"version":          "0.1.0",
		"manifest_version": "3",
		"license":          "MIT",
		"min_core_version": "0.4.0",
		"metadata": map[string]any{
			"description": "test plugin",
			"author":      "raylea",
		},
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
	logicalEntry := "bin/" + pluginID
	targetPlatform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	backendRelative := logicalEntry
	if targetPlatform == "windows-x64" {
		backendRelative += ".exe"
	}
	document := map[string]any{
		"artifact_version": "2", "target_platform": targetPlatform, "entry": backendRelative,
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
	defer func(release func() error) { _ = release() }(input.Close)
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
	templateDir := filepath.Join(pluginRoot, filepath.FromSlash(templatePath))
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("create template directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templateDir, "template.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write placeholder template manifest: %v", err)
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
  "name": "测试模板",
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
	defer func(release func() error) { _ = release() }(file.Close)

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
