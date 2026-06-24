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
        "python": ["python/install/bin/python3"]
      }
    }
  ]
}`
	writeManifest(t, repoRoot, manifest)
	storeRoot := filepath.Join(repoRoot, ".deps", "store", "python-test", "3.12.13", "python", "install", "bin")
	writePreparedFile(t, filepath.Join(storeRoot, "python3"))

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

func TestResolveEntrypointUsesPreparedChromiumBeforeSystemBrowser(t *testing.T) {
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
	preparedPath := filepath.Join(repoRoot, ".deps", "store", "chromium-test", "147.0.7727.24", "chrome-win64", "chrome.exe")
	writePreparedFile(t, preparedPath)

	manager := NewManager(repoRoot)
	manager.findSystemChromium = func(context.Context) (string, error) {
		return filepath.Join(t.TempDir(), "system-chrome.exe"), nil
	}
	manager.downloadFile = func(context.Context, string, string) error {
		t.Fatal("ResolveEntrypoint should not download when prepared chromium exists")
		return nil
	}

	got, err := manager.ResolveEntrypoint(context.Background(), "chromium", "browser")
	if err != nil {
		t.Fatalf("ResolveEntrypoint failed: %v", err)
	}
	if got != preparedPath {
		t.Fatalf("ResolveEntrypoint() = %q, want prepared path %q", got, preparedPath)
	}
}

func TestPrepareWithReportUsesSystemChromiumWithoutDownload(t *testing.T) {
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
	systemPath := filepath.Join(t.TempDir(), "chrome.exe")

	manager := NewManager(repoRoot)
	manager.findSystemChromium = func(context.Context) (string, error) {
		return systemPath, nil
	}
	manager.downloadFile = func(context.Context, string, string) error {
		t.Fatal("PrepareWithReport should not download when system chromium is available")
		return nil
	}

	report, err := manager.PrepareWithReport(context.Background(), "chromium")
	if err != nil {
		t.Fatalf("PrepareWithReport failed: %v", err)
	}
	if !report.UsedSystemBrowser {
		t.Fatalf("expected system chromium report: %#v", report)
	}
	if report.PreparedEntrypoint != systemPath || report.Entrypoints["browser"] != systemPath {
		t.Fatalf("unexpected system browser entrypoint: %#v", report)
	}
	if report.UsedPreparedStore || report.UsedCachedArchive || len(report.AttemptedSources) != 0 || report.SelectedSource != "" {
		t.Fatalf("system chromium should not report managed download state: %#v", report)
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

func TestPrepareWithReportUsesFastestProbedSource(t *testing.T) {
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
          "url": "https://nodejs.org/node.tar.xz",
          "kind": "upstream",
          "label": "nodejs.org"
        },
        {
          "url": "https://mirrors.example.invalid/node.tar.xz",
          "kind": "mirror",
          "label": "mirror"
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
	manager.selectSources = func(_ context.Context, sources []ResourceSource) []ResourceSource {
		return []ResourceSource{sources[1], sources[0]}
	}
	var requested []string
	manager.downloadFile = func(_ context.Context, rawURL string, destPath string) error {
		requested = append(requested, rawURL)
		return os.WriteFile(destPath, []byte("fixture-archive"), 0o644)
	}
	manager.extract = func(_ context.Context, _ string, _ string, destRoot string) error {
		writePreparedFile(t, filepath.Join(destRoot, "node", "bin", "node"))
		writePreparedFile(t, filepath.Join(destRoot, "node", "bin", "npm"))
		return nil
	}
	var events []PrepareProgress

	report, err := manager.PrepareWithReportOptions(context.Background(), "nodejs-runtime", PrepareOptions{
		Progress: func(event PrepareProgress) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("PrepareWithReportOptions failed: %v", err)
	}
	if len(requested) != 1 || requested[0] != "https://mirrors.example.invalid/node.tar.xz" {
		t.Fatalf("expected fastest mirror download first, got %#v", requested)
	}
	if report.SelectedSource != "https://mirrors.example.invalid/node.tar.xz" {
		t.Fatalf("unexpected selected source: %#v", report)
	}
	if !hasPrepareEvent(events, "probe", "running", "") {
		t.Fatalf("expected source probe progress event: %#v", events)
	}
	if !hasPrepareEvent(events, "download", "running", "https://mirrors.example.invalid/node.tar.xz") {
		t.Fatalf("expected mirror download progress event: %#v", events)
	}
}

func TestPrepareWithReportFallsBackToManifestOrderWhenProbeFails(t *testing.T) {
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
	manager.selectSources = func(_ context.Context, _ []ResourceSource) []ResourceSource {
		return nil
	}
	var requested []string
	manager.downloadFile = func(_ context.Context, rawURL string, destPath string) error {
		requested = append(requested, rawURL)
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
	if len(requested) != 1 || requested[0] != "https://primary.example.invalid/node.tar.xz" {
		t.Fatalf("expected manifest order when probes fail, got %#v", requested)
	}
	if report.SelectedSource != "https://primary.example.invalid/node.tar.xz" {
		t.Fatalf("unexpected selected source: %#v", report)
	}
}

func TestPrepareWithReportFallsBackWhenFastestSourceFailsVerification(t *testing.T) {
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
	manager.selectSources = func(_ context.Context, sources []ResourceSource) []ResourceSource {
		return []ResourceSource{sources[1], sources[0]}
	}
	var requested []string
	manager.downloadFile = func(_ context.Context, rawURL string, destPath string) error {
		requested = append(requested, rawURL)
		if strings.Contains(rawURL, "mirror") {
			return os.WriteFile(destPath, []byte("wrong-archive"), 0o644)
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
	if requested[0] != "https://mirror.example.invalid/node.tar.xz" || requested[1] != "https://primary.example.invalid/node.tar.xz" {
		t.Fatalf("unexpected download order: %#v", requested)
	}
	if report.SelectedSource != "https://primary.example.invalid/node.tar.xz" {
		t.Fatalf("unexpected selected source: %#v", report)
	}
}

func TestSelectDownloadSourcesKeepsManifestOrderForCloseProbeSpeeds(t *testing.T) {
	t.Parallel()

	ordered := restoreCloseProbeOrder([]sourceProbeResult{
		{
			source:      ResourceSource{URL: "https://mirror.example.invalid/node.tar.xz"},
			index:       1,
			bytesPerSec: 100,
			ok:          true,
		},
		{
			source:      ResourceSource{URL: "https://primary.example.invalid/node.tar.xz"},
			index:       0,
			bytesPerSec: 95,
			ok:          true,
		},
		{
			source:      ResourceSource{URL: "https://slow.example.invalid/node.tar.xz"},
			index:       2,
			bytesPerSec: 50,
			ok:          true,
		},
	})

	got := []string{ordered[0].source.URL, ordered[1].source.URL, ordered[2].source.URL}
	want := []string{
		"https://primary.example.invalid/node.tar.xz",
		"https://mirror.example.invalid/node.tar.xz",
		"https://slow.example.invalid/node.tar.xz",
	}
	if !slicesEqual(got, want) {
		t.Fatalf("unexpected close probe order: got %#v want %#v", got, want)
	}
}

func TestPrepareWithReportEmitsDownloadFallbackProgress(t *testing.T) {
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
          "kind": "upstream",
          "label": "primary"
        },
        {
          "url": "https://mirror.example.invalid/node.tar.xz",
          "kind": "mirror",
          "label": "mirror"
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
	manager.downloadFile = func(_ context.Context, rawURL string, destPath string) error {
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
	var events []PrepareProgress

	_, err := manager.PrepareWithReportOptions(context.Background(), "nodejs-runtime", PrepareOptions{
		Progress: func(event PrepareProgress) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("PrepareWithReportOptions failed: %v", err)
	}

	if !hasPrepareEvent(events, "download", "running", "https://primary.example.invalid/node.tar.xz") {
		t.Fatalf("expected primary download progress event: %#v", events)
	}
	if !hasPrepareEvent(events, "download", "running", "https://mirror.example.invalid/node.tar.xz") {
		t.Fatalf("expected mirror download progress event: %#v", events)
	}
	if !hasPrepareEvent(events, "complete", "succeeded", "") {
		t.Fatalf("expected completed progress event: %#v", events)
	}
}

func TestPrepareWithReportEmitsCachedArchiveProgress(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	archiveContent := []byte("fixture-archive")
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
      "sha256": "` + sha256Hex(archiveContent) + `",
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
	if err := os.WriteFile(archivePath, archiveContent, 0o644); err != nil {
		t.Fatalf("write cached archive: %v", err)
	}

	manager := NewManager(repoRoot)
	manager.downloadFile = func(context.Context, string, string) error {
		t.Fatal("cached archive should not be downloaded")
		return nil
	}
	manager.extract = func(_ context.Context, _ string, _ string, destRoot string) error {
		writePreparedFile(t, filepath.Join(destRoot, "node", "bin", "node"))
		writePreparedFile(t, filepath.Join(destRoot, "node", "bin", "npm"))
		return nil
	}
	var events []PrepareProgress

	report, err := manager.PrepareWithReportOptions(context.Background(), "nodejs-runtime", PrepareOptions{
		Progress: func(event PrepareProgress) {
			events = append(events, event)
		},
	})
	if err != nil {
		t.Fatalf("PrepareWithReportOptions failed: %v", err)
	}
	if !report.UsedCachedArchive {
		t.Fatalf("expected cached archive report: %#v", report)
	}
	if !hasPrepareEvent(events, "download", "succeeded", "") {
		t.Fatalf("expected cached archive progress event: %#v", events)
	}
}
