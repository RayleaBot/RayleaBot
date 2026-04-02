package deps

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestPlatformNormalizesWindowsAMD64(t *testing.T) {
	t.Parallel()

	if got := ManifestPlatform("windows", "amd64"); got != "windows-x64" {
		t.Fatalf("ManifestPlatform(windows, amd64) = %q, want windows-x64", got)
	}
	if got := ManifestPlatform("darwin", "arm64"); got != "macos-arm64" {
		t.Fatalf("ManifestPlatform(darwin, arm64) = %q, want macos-arm64", got)
	}
}

func TestResourceMetadataCompleteRequiresArchiveAndEntrypoints(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		ID:            "python-windows-x64",
		Kind:          "python-runtime",
		Version:       "3.12.13",
		Platform:      "windows-x64",
		Source:        "https://example.invalid/python.tar.gz",
		SHA256:        "10b9fd9ba9441f246f2cb279c2c6e6b2f98e60ef7960c313fd2bbc7f0c1e6f5e",
		ArchiveFormat: "tar.gz",
		Entrypoints: map[string][]string{
			"python": {"python/install/python.exe"},
			"pip":    {"python/install/Scripts/pip.exe"},
		},
	}
	if !ResourceMetadataComplete(resource) {
		t.Fatalf("expected resource metadata to be complete")
	}

	resource.ArchiveFormat = ""
	if ResourceMetadataComplete(resource) {
		t.Fatalf("resource metadata should require archive_format")
	}
	resource.ArchiveFormat = "tar.gz"
	resource.Entrypoints = map[string][]string{"python": {"python/install/python.exe"}}
	if ResourceMetadataComplete(resource) {
		t.Fatalf("resource metadata should require runtime entrypoints")
	}
}

func TestResolveEntrypointUsesPreparedStoreWithoutDownload(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifest := `{
  "manifest_version": 2,
  "resources": [
    {
      "id": "python-test",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "` + CurrentPlatform() + `",
      "source": "https://example.invalid/python.tar.gz",
      "sha256": "10b9fd9ba9441f246f2cb279c2c6e6b2f98e60ef7960c313fd2bbc7f0c1e6f5e",
      "archive_format": "tar.gz",
      "entrypoints": {
        "python": ["python/install/bin/python3"],
        "pip": ["python/install/bin/pip3"]
      }
    }
  ]
}`
	writeManifest(t, repoRoot, manifest)
	storeRoot := filepath.Join(repoRoot, ".deps", "store", "python-test", "3.12.13", "python", "install", "bin")
	writePreparedFile(t, filepath.Join(storeRoot, "python3"))
	writePreparedFile(t, filepath.Join(storeRoot, "pip3"))

	manager := NewManager(repoRoot)
	downloaded := false
	manager.downloadFile = func(context.Context, string, string) error {
		downloaded = true
		return nil
	}

	command, err := manager.ResolveEntrypoint(context.Background(), "python-runtime", "python")
	if err != nil {
		t.Fatalf("ResolveEntrypoint failed: %v", err)
	}
	if downloaded {
		t.Fatalf("ResolveEntrypoint should not download when prepared store exists")
	}
	if command != filepath.Join(storeRoot, "python3") {
		t.Fatalf("unexpected prepared entrypoint: got %q want %q", command, filepath.Join(storeRoot, "python3"))
	}
}

func TestPrepareDownloadsAndExtractsMissingResource(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifest := `{
  "manifest_version": 2,
  "resources": [
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + CurrentPlatform() + `",
      "source": "https://example.invalid/node.tar.xz",
      "sha256": "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
      "archive_format": "tar.xz",
      "entrypoints": {
        "node": ["node/bin/node"],
        "npm": ["node/bin/npm"]
      }
    }
  ]
}`
	writeManifest(t, repoRoot, manifest)

	manager := NewManager(repoRoot)
	manager.downloadFile = func(_ context.Context, _ string, destPath string) error {
		return os.WriteFile(destPath, []byte("fixture-archive"), 0o644)
	}
	manager.extract = func(_ context.Context, _ string, _ string, destRoot string) error {
		writePreparedFile(t, filepath.Join(destRoot, "node", "bin", "node"))
		writePreparedFile(t, filepath.Join(destRoot, "node", "bin", "npm"))
		return nil
	}

	managerResolve := func() (string, error) {
		return manager.ResolveEntrypoint(context.Background(), "nodejs-runtime", "node")
	}

	path, err := managerResolve()
	if err != nil {
		t.Fatalf("ResolveEntrypoint failed: %v", err)
	}
	wantPath := filepath.Join(repoRoot, ".deps", "store", "node-test", "24.14.0", "node", "bin", "node")
	if path != wantPath {
		t.Fatalf("unexpected managed node path: got %q want %q", path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("managed node entrypoint should be prepared: %v", err)
	}
}

func TestInspectReportsCachedArchiveAndPreparedStore(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifest := `{
  "manifest_version": 2,
  "resources": [
    {
      "id": "chromium-test",
      "kind": "chromium",
      "version": "147.0.7727.24",
      "platform": "` + CurrentPlatform() + `",
      "source": "https://example.invalid/chromium.zip",
      "sha256": "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
      "archive_format": "zip",
      "entrypoints": {
        "browser": ["chrome-win64/chrome.exe"]
      }
    }
  ]
}`
	writeManifest(t, repoRoot, manifest)

	cachePath := filepath.Join(CacheRoot(repoRoot), "chromium-test-147.0.7727.24.zip")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache root: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("fixture-archive"), 0o644); err != nil {
		t.Fatalf("write cached archive: %v", err)
	}
	writePreparedFile(t, filepath.Join(StoreRoot(repoRoot, &Resource{ID: "chromium-test", Version: "147.0.7727.24"}), "chrome-win64", "chrome.exe"))

	manager := NewManager(repoRoot)
	inspection, err := manager.Inspect("chromium")
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}

	if !inspection.MetadataComplete {
		t.Fatalf("expected metadata complete inspection: %#v", inspection)
	}
	if !inspection.CachedArchivePresent {
		t.Fatalf("expected cached archive to be detected: %#v", inspection)
	}
	if !inspection.PreparedStorePresent {
		t.Fatalf("expected prepared store to be detected: %#v", inspection)
	}
}

func TestPrepareWithReportClassifiesDownloadFailure(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifest := `{
  "manifest_version": 2,
  "resources": [
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + CurrentPlatform() + `",
      "source": "https://example.invalid/node.tar.xz",
      "sha256": "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
      "archive_format": "tar.xz",
      "entrypoints": {
        "node": ["node/bin/node"],
        "npm": ["node/bin/npm"]
      }
    }
  ]
}`
	writeManifest(t, repoRoot, manifest)

	manager := NewManager(repoRoot)
	manager.downloadFile = func(context.Context, string, string) error {
		return errors.New("offline")
	}

	_, err := manager.PrepareWithReport(context.Background(), "nodejs-runtime")
	if err == nil {
		t.Fatal("PrepareWithReport should fail when download fails")
	}

	var bootstrapErr *BootstrapError
	if !errors.As(err, &bootstrapErr) {
		t.Fatalf("expected BootstrapError, got %T: %v", err, err)
	}
	if bootstrapErr.Kind != "nodejs-runtime" {
		t.Fatalf("unexpected error kind: %#v", bootstrapErr)
	}
	if bootstrapErr.Stage != "download" {
		t.Fatalf("unexpected error stage: %#v", bootstrapErr)
	}
	if bootstrapErr.ArchivePath == "" || bootstrapErr.StoreRoot == "" {
		t.Fatalf("expected archive/store paths in BootstrapError: %#v", bootstrapErr)
	}
	if bootstrapErr.Remediation == "" {
		t.Fatalf("expected remediation in BootstrapError: %#v", bootstrapErr)
	}
}

func writeManifest(t *testing.T, repoRoot, content string) {
	t.Helper()
	path := filepath.Join(repoRoot, ".deps", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir manifest root: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func writePreparedFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir prepared file dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write prepared file: %v", err)
	}
}
