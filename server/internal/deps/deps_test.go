package deps

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestPlatformNormalizesSupportedArchitectures(t *testing.T) {
	t.Parallel()
	if got := ManifestPlatform("windows", "amd64"); got != "windows-x64" {
		t.Fatalf("ManifestPlatform(windows, amd64) = %q", got)
	}
	if got := ManifestPlatform("darwin", "arm64"); got != "macos-arm64" {
		t.Fatalf("ManifestPlatform(darwin, arm64) = %q", got)
	}
}

func TestLoadManifestRejectsLegacyVersionAndRuntimeKinds(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	resource := testChromiumResource("fixture")
	writeDepsManifest(t, repoRoot, 3, resource)
	if _, err := LoadManifest(repoRoot); err == nil {
		t.Fatal("deps manifest v3 must be rejected")
	}
	resource.Kind = "python-runtime"
	writeDepsManifest(t, repoRoot, ManifestVersion, resource)
	if _, err := LoadManifest(repoRoot); err == nil {
		t.Fatal("non-Chromium managed dependency must be rejected")
	}
}

func TestResourceMetadataCompleteRequiresChromiumInputs(t *testing.T) {
	t.Parallel()
	resource := testChromiumResource("fixture")
	if !ResourceMetadataComplete(&resource) {
		t.Fatal("complete Chromium metadata was rejected")
	}
	resource.ArchiveFormat = ""
	if ResourceMetadataComplete(&resource) {
		t.Fatal("archive_format is required")
	}
	resource = testChromiumResource("fixture")
	resource.Entrypoints = map[string][]string{}
	if ResourceMetadataComplete(&resource) {
		t.Fatal("browser entrypoint is required")
	}
	resource = testChromiumResource("fixture")
	resource.Sources = append(resource.Sources, resource.Sources[0])
	if ResourceMetadataComplete(&resource) {
		t.Fatal("duplicate source URLs must be rejected")
	}
}

func TestResolveEntrypointUsesPreparedChromiumWithoutDownload(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	resource := testChromiumResource("fixture")
	writeDepsManifest(t, repoRoot, ManifestVersion, resource)
	preparedPath := filepath.Join(StoreRoot(repoRoot, &resource), "chromium", "chrome")
	writePreparedFile(t, preparedPath)

	manager := NewManager(repoRoot)
	manager.findSystemChromium = func(context.Context) (string, error) { return "", errors.New("not installed") }
	manager.downloadFile = func(context.Context, string, string) error {
		t.Fatal("prepared Chromium must not be downloaded")
		return nil
	}
	got, err := manager.ResolveEntrypoint(context.Background(), "chromium", "browser")
	if err != nil {
		t.Fatalf("ResolveEntrypoint: %v", err)
	}
	if got != preparedPath {
		t.Fatalf("entrypoint = %q, want %q", got, preparedPath)
	}
}

func TestPrepareWithReportUsesSystemChromiumWithoutDownload(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	writeDepsManifest(t, repoRoot, ManifestVersion, testChromiumResource("fixture"))
	systemPath := filepath.Join(t.TempDir(), "chrome.exe")
	manager := NewManager(repoRoot)
	manager.findSystemChromium = func(context.Context) (string, error) { return systemPath, nil }
	manager.downloadFile = func(context.Context, string, string) error {
		t.Fatal("system Chromium must not trigger a download")
		return nil
	}
	report, err := manager.PrepareWithReport(context.Background(), "chromium")
	if err != nil {
		t.Fatalf("PrepareWithReport: %v", err)
	}
	if !report.UsedSystemBrowser || report.PreparedEntrypoint != systemPath {
		t.Fatalf("unexpected system Chromium report: %#v", report)
	}
}

func TestPrepareDownloadsVerifiesAndExtractsChromium(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	archive := []byte("verified fixture archive")
	resource := testChromiumResource(sha256Hex(archive))
	writeDepsManifest(t, repoRoot, ManifestVersion, resource)

	manager := NewManager(repoRoot)
	manager.findSystemChromium = func(context.Context) (string, error) { return "", errors.New("not installed") }
	manager.downloadFile = func(_ context.Context, _ string, destination string) error {
		return os.WriteFile(destination, archive, 0o644)
	}
	manager.extract = func(_ context.Context, _, _ string, destination string) error {
		writePreparedFile(t, filepath.Join(destination, "chromium", "chrome"))
		return nil
	}

	report, err := manager.PrepareWithReport(context.Background(), "chromium")
	if err != nil {
		t.Fatalf("PrepareWithReport: %v", err)
	}
	want := filepath.Join(StoreRoot(repoRoot, &resource), "chromium", "chrome")
	if report.PreparedEntrypoint != want || report.SelectedSource != resource.Sources[0].URL {
		t.Fatalf("unexpected prepared report: %#v", report)
	}
}

func TestPrepareFallsBackToNextChromiumSource(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	archive := []byte("verified fixture archive")
	resource := testChromiumResource(sha256Hex(archive))
	resource.Sources = []ResourceSource{
		{URL: "https://primary.example.invalid/chromium.zip", Kind: "upstream"},
		{URL: "https://mirror.example.invalid/chromium.zip", Kind: "mirror"},
	}
	writeDepsManifest(t, repoRoot, ManifestVersion, resource)

	manager := NewManager(repoRoot)
	manager.findSystemChromium = func(context.Context) (string, error) { return "", errors.New("not installed") }
	manager.selectSources = func(_ context.Context, sources []ResourceSource) []ResourceSource { return sources }
	manager.downloadFile = func(_ context.Context, source, destination string) error {
		if source == resource.Sources[0].URL {
			return errors.New("primary unavailable")
		}
		return os.WriteFile(destination, archive, 0o644)
	}
	manager.extract = func(_ context.Context, _, _ string, destination string) error {
		writePreparedFile(t, filepath.Join(destination, "chromium", "chrome"))
		return nil
	}
	var events []PrepareProgress
	report, err := manager.PrepareWithReportOptions(context.Background(), "chromium", PrepareOptions{Progress: func(event PrepareProgress) {
		events = append(events, event)
	}})
	if err != nil {
		t.Fatalf("PrepareWithReportOptions: %v", err)
	}
	if report.SelectedSource != resource.Sources[1].URL || len(report.AttemptedSources) != 2 {
		t.Fatalf("source fallback report = %#v", report)
	}
	if !hasPrepareEvent(events, "download", "running", resource.Sources[0].URL) || !hasPrepareEvent(events, "complete", "succeeded", "") {
		t.Fatalf("missing fallback progress events: %#v", events)
	}
}

func TestInspectReportsChromiumCacheAndPreparedStore(t *testing.T) {
	t.Parallel()
	repoRoot := t.TempDir()
	resource := testChromiumResource("fixture")
	writeDepsManifest(t, repoRoot, ManifestVersion, resource)
	archivePath := filepath.Join(CacheRoot(repoRoot), resource.ID+"-"+resource.Version+".zip")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archivePath, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	writePreparedFile(t, filepath.Join(StoreRoot(repoRoot, &resource), "chromium", "chrome"))

	manager := NewManager(repoRoot)
	manager.findSystemChromium = func(context.Context) (string, error) { return "", errors.New("not installed") }
	inspection, err := manager.Inspect("chromium")
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !inspection.MetadataComplete || !inspection.CachedArchivePresent || !inspection.PreparedStorePresent {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}
}

func testChromiumResource(hash string) Resource {
	if hash == "fixture" {
		hash = sha256Hex([]byte("fixture"))
	}
	return Resource{
		ID: "chromium-test", Kind: "chromium", Version: "147.0.0", Platform: CurrentPlatform(),
		Sources: []ResourceSource{{URL: "https://example.invalid/chromium.zip", Kind: "upstream"}},
		SHA256: hash, ArchiveFormat: "zip",
		Entrypoints: map[string][]string{"browser": {"chromium/chrome"}},
	}
}

func writeDepsManifest(t *testing.T, repoRoot string, version int, resource Resource) {
	t.Helper()
	content, err := json.MarshalIndent(Manifest{ManifestVersion: version, Resources: []Resource{resource}}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeManifest(t, repoRoot, string(content)+"\n")
}
