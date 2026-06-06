package deps

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
		ID:       "python-windows-x64",
		Kind:     "python-runtime",
		Version:  "3.12.13",
		Platform: "windows-x64",
		Sources: []ResourceSource{
			{URL: "https://example.invalid/python.tar.gz", Kind: "upstream"},
		},
		SHA256:        "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
		ArchiveFormat: "tar.gz",
		Entrypoints: map[string][]string{
			"python": {"python/python.exe"},
			"pip":    {"python/Scripts/pip.exe"},
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
	resource.Entrypoints = map[string][]string{}
	if ResourceMetadataComplete(resource) {
		t.Fatalf("resource metadata should require runtime entrypoints")
	}
	resource.Entrypoints = map[string][]string{
		"python": {"python/python.exe"},
		"pip":    {"python/Scripts/pip.exe"},
	}
	resource.Sources = []ResourceSource{
		{URL: "https://example.invalid/python.tar.gz", Kind: "upstream"},
		{URL: "https://example.invalid/python.tar.gz", Kind: "mirror"},
	}
	if ResourceMetadataComplete(resource) {
		t.Fatalf("resource metadata should reject duplicate source URLs")
	}
	resource.Sources = nil
	if ResourceMetadataComplete(resource) {
		t.Fatalf("resource metadata should require at least one source")
	}
	resource.Sources = []ResourceSource{
		{URL: "https://example.invalid/python.tar.gz", Kind: "internal"},
	}
	if ResourceMetadataComplete(resource) {
		t.Fatalf("resource metadata should reject unknown source kinds")
	}
}

func TestResolveEntrypointUsesPreparedStoreWithoutDownload(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifest := `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "python-test",
      "kind": "python-runtime",
      "version": "3.12.13",
      "platform": "` + CurrentPlatform() + `",
      "sources": [
        {
          "url": "https://example.invalid/python.tar.gz",
          "kind": "upstream"
        }
      ],
      "sha256": "10b7a95b928e551fc78cac665999e1ae1f08fb738b255adb0a8d3b9c2824a9c0",
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
  "manifest_version": 3,
  "resources": [
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + CurrentPlatform() + `",
      "sources": [
        {
          "url": "https://example.invalid/node.tar.xz",
          "kind": "upstream"
        }
      ],
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
  "manifest_version": 3,
  "resources": [
    {
      "id": "chromium-test",
      "kind": "chromium",
      "version": "147.0.7727.24",
      "platform": "` + CurrentPlatform() + `",
      "sources": [
        {
          "url": "https://example.invalid/chromium.zip",
          "kind": "upstream"
        }
      ],
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
  "manifest_version": 3,
  "resources": [
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + CurrentPlatform() + `",
      "sources": [
        {
          "url": "https://example.invalid/node.tar.xz",
          "kind": "upstream"
        },
        {
          "url": "https://mirror.example.invalid/node.tar.xz",
          "kind": "mirror"
        }
      ],
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
	if len(bootstrapErr.AttemptedSources) != 2 {
		t.Fatalf("expected attempted sources in BootstrapError: %#v", bootstrapErr)
	}
}

func TestPrepareWithReportFallsBackToNextSource(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifest := `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + CurrentPlatform() + `",
      "sources": [
        {
          "url": "https://primary.example.invalid/node.tar.xz",
          "kind": "upstream"
        },
        {
          "url": "https://mirror.example.invalid/node.tar.xz",
          "kind": "mirror"
        }
      ],
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
	var requested []string
	manager.downloadFile = func(_ context.Context, rawURL string, destPath string) error {
		requested = append(requested, rawURL)
		if strings.Contains(rawURL, "primary") {
			return errors.New("offline")
		}
		return os.WriteFile(destPath, []byte("fixture-archive"), 0o644)
	}
	manager.extract = func(_ context.Context, _ string, _ string, destRoot string) error {
		writePreparedFile(t, filepath.Join(destRoot, "node", "bin", "node"))
		writePreparedFile(t, filepath.Join(destRoot, "node", "bin", "npm"))
		return nil
	}

	report, err := manager.PrepareWithReport(context.Background(), "nodejs-runtime")
	if err != nil {
		t.Fatalf("PrepareWithReport failed: %v", err)
	}
	if len(requested) != 2 {
		t.Fatalf("expected two download attempts, got %#v", requested)
	}
	if report.SelectedSource != "https://mirror.example.invalid/node.tar.xz" {
		t.Fatalf("unexpected selected source: %#v", report)
	}
	if len(report.AttemptedSources) != 2 {
		t.Fatalf("expected attempted sources in report: %#v", report)
	}
}

func TestPrepareWithReportUsesCachedArchiveWithoutDownload(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifest := `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "node-test",
      "kind": "nodejs-runtime",
      "version": "24.14.0",
      "platform": "` + CurrentPlatform() + `",
      "sources": [
        {
          "url": "https://example.invalid/node.tar.xz",
          "kind": "upstream"
        }
      ],
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

	archivePath := filepath.Join(CacheRoot(repoRoot), "node-test-24.14.0.tar.xz")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("mkdir cache root: %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("fixture-archive"), 0o644); err != nil {
		t.Fatalf("write cached archive: %v", err)
	}

	manager := NewManager(repoRoot)
	downloaded := false
	manager.downloadFile = func(context.Context, string, string) error {
		downloaded = true
		return nil
	}
	manager.extract = func(_ context.Context, _ string, _ string, destRoot string) error {
		writePreparedFile(t, filepath.Join(destRoot, "node", "bin", "node"))
		writePreparedFile(t, filepath.Join(destRoot, "node", "bin", "npm"))
		return nil
	}

	report, err := manager.PrepareWithReport(context.Background(), "nodejs-runtime")
	if err != nil {
		t.Fatalf("PrepareWithReport failed: %v", err)
	}
	if downloaded {
		t.Fatal("PrepareWithReport should not download when cached archive matches")
	}
	if !report.UsedCachedArchive {
		t.Fatalf("expected cached archive hit, got %#v", report)
	}
	if len(report.AttemptedSources) != 0 || report.SelectedSource != "" {
		t.Fatalf("cached archive should not record source attempts, got %#v", report)
	}
}

func TestPrepareWithReportCleansStaleTempRootBeforeExtractingCachedArchive(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifest := `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "chromium-test",
      "kind": "chromium",
      "version": "147.0.7727.24",
      "platform": "` + CurrentPlatform() + `",
      "sources": [
        {
          "url": "https://example.invalid/chromium.zip",
          "kind": "upstream"
        }
      ],
      "sha256": "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
      "archive_format": "zip",
      "entrypoints": {
        "browser": ["chrome-win64/chrome.exe"]
      }
    }
  ]
}`
	writeManifest(t, repoRoot, manifest)

	resource := &Resource{ID: "chromium-test", Version: "147.0.7727.24"}
	storeParent := filepath.Dir(StoreRoot(repoRoot, resource))
	staleRoot := filepath.Join(storeParent, ".chromium-test-147.0.7727.24-stale")
	writePreparedFile(t, filepath.Join(staleRoot, "chrome-win64", "chrome.exe"))

	archivePath := filepath.Join(CacheRoot(repoRoot), "chromium-test-147.0.7727.24.zip")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("mkdir cache root: %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("fixture-archive"), 0o644); err != nil {
		t.Fatalf("write cached archive: %v", err)
	}

	manager := NewManager(repoRoot)
	manager.downloadFile = func(context.Context, string, string) error {
		t.Fatal("PrepareWithReport should not download when cached archive matches")
		return nil
	}
	manager.extract = func(_ context.Context, _ string, _ string, destRoot string) error {
		if destRoot == staleRoot {
			t.Fatal("extract should use a fresh temp root")
		}
		writePreparedFile(t, filepath.Join(destRoot, "chrome-win64", "chrome.exe"))
		return nil
	}

	report, err := manager.PrepareWithReport(context.Background(), "chromium")
	if err != nil {
		t.Fatalf("PrepareWithReport failed: %v", err)
	}
	wantPath := filepath.Join(repoRoot, ".deps", "store", "chromium-test", "147.0.7727.24", "chrome-win64", "chrome.exe")
	if report.PreparedEntrypoint != wantPath {
		t.Fatalf("prepared entrypoint = %q, want %q", report.PreparedEntrypoint, wantPath)
	}
	if _, err := os.Stat(staleRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale temp root should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("official store entrypoint should exist: %v", err)
	}
}

func TestPrepareWithReportRemovesIncompleteStoreRootBeforeExtractingCachedArchive(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifest := `{
  "manifest_version": 3,
  "resources": [
    {
      "id": "chromium-test",
      "kind": "chromium",
      "version": "147.0.7727.24",
      "platform": "` + CurrentPlatform() + `",
      "sources": [
        {
          "url": "https://example.invalid/chromium.zip",
          "kind": "upstream"
        }
      ],
      "sha256": "2bb9e071b229e9c0cb7d90297c51fa4cf3f5dbf4f88aded36d3f5892651baabf",
      "archive_format": "zip",
      "entrypoints": {
        "browser": ["chrome-win64/chrome.exe"]
      }
    }
  ]
}`
	writeManifest(t, repoRoot, manifest)

	resource := &Resource{ID: "chromium-test", Kind: "chromium", Version: "147.0.7727.24"}
	storeRoot := StoreRoot(repoRoot, resource)
	writePreparedFile(t, filepath.Join(storeRoot, "chrome-win64", "chrome.dll"))

	archivePath := filepath.Join(CacheRoot(repoRoot), "chromium-test-147.0.7727.24.zip")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("mkdir cache root: %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("fixture-archive"), 0o644); err != nil {
		t.Fatalf("write cached archive: %v", err)
	}

	manager := NewManager(repoRoot)
	manager.downloadFile = func(context.Context, string, string) error {
		t.Fatal("PrepareWithReport should not download when cached archive matches")
		return nil
	}
	manager.extract = func(_ context.Context, _ string, _ string, destRoot string) error {
		if _, err := os.Stat(storeRoot); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("incomplete store root should be removed before extraction, stat err = %v", err)
		}
		writePreparedFile(t, filepath.Join(destRoot, "chrome-win64", "chrome.exe"))
		return nil
	}

	report, err := manager.PrepareWithReport(context.Background(), "chromium")
	if err != nil {
		t.Fatalf("PrepareWithReport failed: %v", err)
	}
	wantPath := filepath.Join(storeRoot, "chrome-win64", "chrome.exe")
	if report.PreparedEntrypoint != wantPath {
		t.Fatalf("prepared entrypoint = %q, want %q", report.PreparedEntrypoint, wantPath)
	}
	if _, err := os.Stat(filepath.Join(storeRoot, "chrome-win64", "chrome.dll")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("incomplete store file should be removed, stat err = %v", err)
	}
}

func TestRepoWindowsPythonManifestEntrypointsMatchPreparedLayout(t *testing.T) {
	t.Parallel()

	if CurrentPlatform() != "windows-x64" {
		t.Skip("windows python manifest layout is only checked on windows-x64")
	}

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(workingDir, "..", "..", ".."))

	manifest, err := LoadManifest(repoRoot)
	if err != nil {
		t.Fatalf("load repo manifest: %v", err)
	}
	resource := manifest.FindResource("windows-x64", "python-runtime")
	if resource == nil {
		t.Fatal("repo manifest does not include windows python runtime resource")
	}

	storeRoot := StoreRoot(repoRoot, resource)
	if _, err := os.Stat(storeRoot); err == nil {
		entrypoints, err := resolvePreparedEntrypoints(storeRoot, resource)
		if err != nil {
			t.Fatalf("resolve repo python entrypoints from %s failed: %v", storeRoot, err)
		}
		if strings.TrimSpace(entrypoints["python"]) == "" {
			t.Fatalf("resolved repo python entrypoints are incomplete: %#v", entrypoints)
		}
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect repo python store root %s: %v", storeRoot, err)
	}

	syntheticStoreRoot := t.TempDir()
	for _, key := range []string{"python"} {
		candidates := resource.Entrypoints[key]
		if len(candidates) == 0 {
			t.Fatalf("repo manifest is missing %s entrypoints for windows python runtime", key)
		}
		writePreparedFile(t, filepath.Join(syntheticStoreRoot, filepath.FromSlash(candidates[0])))
	}

	entrypoints, err := resolvePreparedEntrypoints(syntheticStoreRoot, resource)
	if err != nil {
		t.Fatalf("resolve repo python entrypoints from synthetic layout failed: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(entrypoints["python"]), "python/python.exe") {
		t.Fatalf("unexpected python entrypoint: %q", entrypoints["python"])
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
