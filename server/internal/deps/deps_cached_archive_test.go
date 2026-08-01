package deps

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPrepareWithReportUsesCachedArchiveWithoutDownload(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	resource := testChromiumResource(sha256Hex([]byte("fixture-archive")))
	writeDepsManifest(t, repoRoot, ManifestVersion, resource)

	archivePath := filepath.Join(CacheRoot(repoRoot), resource.ID+"-"+resource.Version+".zip")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("mkdir cache root: %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("fixture-archive"), 0o644); err != nil {
		t.Fatalf("write cached archive: %v", err)
	}

	manager := NewManager(repoRoot)
	manager.findSystemChromium = func(context.Context) (string, error) { return "", errors.New("not installed") }
	downloaded := false
	manager.downloadFile = func(context.Context, string, string) error {
		downloaded = true
		return nil
	}
	manager.extract = func(_ context.Context, _ string, _ string, destRoot string) error {
		writePreparedFile(t, filepath.Join(destRoot, "chromium", "chrome"))
		return nil
	}

	report, err := manager.PrepareWithReport(context.Background(), "chromium")
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

func TestExtractZipReportsEntryProgress(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "runtime.zip")
	writeZipArchive(t, archivePath, map[string]string{
		"chromium/chrome":    "binary",
		"chromium/README.txt": "readme",
	})
	var events []extractProgress

	if err := ZipWithProgress(archivePath, t.TempDir(), func(event ExtractProgress) {
		events = append(events, extractProgress{
			ExtractedEntries: event.ExtractedEntries,
			TotalEntries:     event.TotalEntries,
			Progress:         event.Progress,
		})
	}); err != nil {
		t.Fatalf("extractZipWithProgress failed: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("expected zip extract progress events")
	}
	last := events[len(events)-1]
	if last.TotalEntries != 2 || last.ExtractedEntries != 2 || last.Progress != 100 {
		t.Fatalf("unexpected final zip progress: %#v", last)
	}
}

func TestExtractTarGzReportsEntryProgress(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "runtime.tar.gz")
	writeTarGzArchive(t, archivePath, map[string]string{
		"chromium/chrome":     "binary",
		"chromium/README.txt": "readme",
	})
	var events []extractProgress

	if err := TarGzWithProgress(archivePath, t.TempDir(), func(event ExtractProgress) {
		events = append(events, extractProgress{
			ExtractedEntries: event.ExtractedEntries,
			TotalEntries:     event.TotalEntries,
			Progress:         event.Progress,
		})
	}); err != nil {
		t.Fatalf("extractTarGzWithProgress failed: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("expected tar.gz extract progress events")
	}
	last := events[len(events)-1]
	if last.TotalEntries != 2 || last.ExtractedEntries != 2 || last.Progress != 100 {
		t.Fatalf("unexpected final tar.gz progress: %#v", last)
	}
}

func TestPrepareStoreCoalescesExtractProgressByPercentage(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	archivePath := filepath.Join(t.TempDir(), "runtime.zip")
	files := map[string]string{
		"chromium/chrome": "binary",
	}
	for index := 0; index < 250; index++ {
		files[fmt.Sprintf("chromium/lib/file-%03d.txt", index)] = "fixture"
	}
	writeZipArchive(t, archivePath, files)
	resource := Resource{
		ID:            "chromium-test",
		Kind:          "chromium",
		Version:       "147.0.0",
		ArchiveFormat: "zip",
		Entrypoints: map[string][]string{
			"browser": {"chromium/chrome"},
		},
	}
	var progresses []int
	err := ensurePreparedResourceWithProgress(
		context.Background(),
		repoRoot,
		resource,
		archivePath,
		nil,
		func(event PrepareProgress) {
			if event.Stage == "extract" && event.Status == "running" && event.TotalEntries > 0 {
				progresses = append(progresses, event.Progress)
			}
		},
	)
	if err != nil {
		t.Fatalf("ensure prepared resource: %v", err)
	}
	if len(progresses) == 0 || len(progresses) > 101 {
		t.Fatalf("extract progress event count = %d, want 1..101", len(progresses))
	}
	for index := 1; index < len(progresses); index++ {
		if progresses[index] == progresses[index-1] {
			t.Fatalf("duplicate extract progress percentage at index %d: %d", index, progresses[index])
		}
	}
	if progresses[len(progresses)-1] != 100 {
		t.Fatalf("final extract progress = %d, want 100", progresses[len(progresses)-1])
	}
}

func TestAcquireLockReclaimsLockFromExitedProcess(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "platform.lock")
	if err := os.WriteFile(lockPath, []byte("4242 2026-07-10T00:00:00Z\n"), 0o644); err != nil {
		t.Fatalf("write abandoned lock: %v", err)
	}
	checkedPID := 0
	release, err := acquireLockWithOwnerCheck(
		context.Background(),
		lockPath,
		time.Now,
		func(pid int) bool {
			checkedPID = pid
			return false
		},
	)
	if err != nil {
		t.Fatalf("reclaim abandoned lock: %v", err)
	}
	defer release()
	if checkedPID != 4242 {
		t.Fatalf("checked lock owner pid = %d, want 4242", checkedPID)
	}
	ownerPID, ok := readLockOwnerPID(lockPath)
	if !ok || ownerPID != os.Getpid() {
		t.Fatalf("reclaimed lock owner = %d, %v; want current pid %d", ownerPID, ok, os.Getpid())
	}
}

func TestProcessIsRunningDistinguishesCurrentAndExitedProcess(t *testing.T) {
	t.Parallel()

	if !processIsRunning(os.Getpid()) {
		t.Fatal("current process should be reported as running")
	}
	command := exec.Command(os.Args[0], "-test.run=^$")
	if err := command.Start(); err != nil {
		t.Fatalf("start short-lived process: %v", err)
	}
	pid := command.Process.Pid
	if err := command.Wait(); err != nil {
		t.Fatalf("wait for short-lived process: %v", err)
	}
	if processIsRunning(pid) {
		t.Fatalf("exited process %d should not be reported as running", pid)
	}
}

func TestPrepareWithReportCleansStaleTempRootBeforeExtractingCachedArchive(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	manifest := `{
  "manifest_version": 4,
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
	manager.findSystemChromium = func(context.Context) (string, error) {
		return "", errors.New("system chromium unavailable")
	}
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
  "manifest_version": 4,
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
	manager.findSystemChromium = func(context.Context) (string, error) {
		return "", errors.New("system chromium unavailable")
	}
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

func TestRepoManifestContainsOnlyChromiumWithResolvableBrowserEntrypoint(t *testing.T) {
	t.Parallel()

	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(workingDir, "..", "..", ".."))

	manifest, err := LoadManifest(repoRoot)
	if err != nil {
		t.Fatalf("load repo manifest: %v", err)
	}
	for _, declared := range manifest.Resources {
		if declared.Kind != "chromium" {
			t.Fatalf("repo manifest contains retired resource kind %q", declared.Kind)
		}
	}
	resource := manifest.FindResource(CurrentPlatform(), "chromium")
	if resource == nil {
		t.Fatalf("repo manifest does not include Chromium for %s", CurrentPlatform())
	}

	storeRoot := StoreRoot(repoRoot, resource)
	if _, err := os.Stat(storeRoot); err == nil {
		entrypoints, err := resolvePreparedEntrypoints(storeRoot, resource)
		if err != nil {
			t.Fatalf("resolve repo Chromium entrypoint from %s failed: %v", storeRoot, err)
		}
		if entrypoints["browser"] == "" {
			t.Fatalf("resolved repo Chromium entrypoint is incomplete: %#v", entrypoints)
		}
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect repo Chromium store root %s: %v", storeRoot, err)
	}

	syntheticStoreRoot := t.TempDir()
	candidates := resource.Entrypoints["browser"]
	if len(candidates) == 0 {
		t.Fatal("repo manifest is missing the Chromium browser entrypoint")
	}
	writePreparedFile(t, filepath.Join(syntheticStoreRoot, filepath.FromSlash(candidates[0])))

	entrypoints, err := resolvePreparedEntrypoints(syntheticStoreRoot, resource)
	if err != nil {
		t.Fatalf("resolve repo Chromium entrypoint from synthetic layout failed: %v", err)
	}
	if entrypoints["browser"] == "" {
		t.Fatalf("unexpected Chromium entrypoint: %q", entrypoints["browser"])
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

func hasPrepareEvent(events []PrepareProgress, stage, status, sourceURL string) bool {
	for _, event := range events {
		if event.Stage != stage || event.Status != status {
			continue
		}
		if sourceURL != "" && event.SourceURL != sourceURL {
			continue
		}
		return true
	}
	return false
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func slicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func writeZipArchive(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("mkdir zip root: %v", err)
	}
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(out)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}

func writeTarGzArchive(t *testing.T, archivePath string, files map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("mkdir tar.gz root: %v", err)
	}
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range files {
		payload := []byte(content)
		if err := tarWriter.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(payload)),
		}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write(payload); err != nil {
			t.Fatalf("write tar entry: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := os.WriteFile(archivePath, buffer.Bytes(), 0o644); err != nil {
		t.Fatalf("write tar.gz archive: %v", err)
	}
}
