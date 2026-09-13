package releaseupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type releaseFile struct {
	name    string
	payload string
	mode    os.FileMode
}

const (
	testArchiveURL = "https://example.com/releases/download/v1.0.0/"
	testRootName   = "RayleaBot-v1.0.0-windows-x64-full/"
)

// releaseArchive builds a release archive whose root folder names version 1.0.0
// and whose build_info.json reports buildVersion.
func releaseArchive(t *testing.T, format, buildVersion string, method uint16, files []releaseFile) []byte {
	t.Helper()
	buildInfo, err := json.Marshal(BuildInfo{Version: buildVersion, GitCommit: "abcdef1", ArtifactID: ArtifactWindowsX64Full})
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, releaseFile{name: "build_info.json", payload: string(buildInfo), mode: 0o644})
	var buffer bytes.Buffer
	switch format {
	case "zip":
		writer := zip.NewWriter(&buffer)
		for _, file := range files {
			header := &zip.FileHeader{Name: testRootName + file.name, Method: method}
			header.SetMode(file.mode)
			stream, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(stream, file.payload); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	case "tar.gz":
		compressed := gzip.NewWriter(&buffer)
		writer := tar.NewWriter(compressed)
		for _, file := range files {
			header := &tar.Header{Name: testRootName + file.name, Mode: int64(file.mode), Size: int64(len(file.payload)), Typeflag: tar.TypeReg}
			if err := writer.WriteHeader(header); err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(writer, file.payload); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := compressed.Close(); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unsupported format %s", format)
	}
	return buffer.Bytes()
}

// releaseChecker serves a 1.0.0 manifest and the given archive, counting archive downloads.
func releaseChecker(t *testing.T, format string, archive []byte, mutate func(*Artifact)) (*Checker, *int) {
	t.Helper()
	fileName := "RayleaBot-v1.0.0-windows-x64-full." + format
	artifact := Artifact{ArtifactID: ArtifactWindowsX64Full, FileName: fileName, DownloadURL: testArchiveURL + fileName, ArchiveSizeBytes: int64(len(archive)), UpdateMode: "guided"}
	if mutate != nil {
		mutate(&artifact)
	}
	manifest, err := json.Marshal(Manifest{Version: "1.0.0", ReleaseNotesRef: "https://example.com/releases/v1.0.0", Artifacts: []Artifact{artifact}})
	if err != nil {
		t.Fatal(err)
	}
	downloads := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := manifest
		if strings.HasPrefix(request.URL.String(), testArchiveURL) {
			downloads++
			body = archive
		}
		return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(body)), Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header), Request: request}, nil
	})}
	checker := NewChecker()
	checker.HTTPClient = client
	checker.DownloadClient = client
	return checker, &downloads
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, payload := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func assertTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, want := range files {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q, %v; want %q", name, got, err, want)
		}
	}
}

func assertAbsent(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still exists: %v", name, err)
		}
	}
}

func installedRoot(t *testing.T, version string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "build_info.json"), marshalBuildInfo(t, testBuildInfo(version)))
	writeTree(t, root, map[string]string{
		"raylea-server.exe":             "old server",
		"web/dist/index.html":           "old index",
		"web/dist/assets/old.js":        "old asset",
		".deps/manifest.json":           "old deps manifest",
		".deps/store/chromium/1/chrome": "runtime",
		"config/user.yaml":              "user config",
		"data/rayleabot.db":             "database",
		"plugins/installed/p/info.json": "plugin",
	})
	return root
}

var newRelease = []releaseFile{
	{name: "raylea-server.exe", payload: "new server", mode: 0o755},
	{name: "web/dist/index.html", payload: "new index", mode: 0o644},
	{name: ".deps/manifest.json", payload: "new deps manifest", mode: 0o644},
	{name: "templates/help.menu/template.json", payload: "template", mode: 0o644},
}

const archiveCache = "cache/downloads/update/RayleaBot-v1.0.0-windows-x64-full.zip"

func TestApplyReplacesReleaseFilesAndKeepsRuntimeData(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	checker, downloads := releaseChecker(t, "zip", releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease), nil)
	result, err := checker.Apply(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "update_available" || InstalledVersion(root) != "1.0.0" || *downloads != 1 {
		t.Fatalf("result = %#v installed = %s downloads = %d", result, InstalledVersion(root), *downloads)
	}
	assertTree(t, root, map[string]string{
		"raylea-server.exe":                 "new server",
		"web/dist/index.html":               "new index",
		".deps/manifest.json":               "new deps manifest",
		"templates/help.menu/template.json": "template",
		"web/dist/assets/old.js":            "old asset",
		".deps/store/chromium/1/chrome":     "runtime",
		"config/user.yaml":                  "user config",
		"data/rayleabot.db":                 "database",
		"plugins/installed/p/info.json":     "plugin",
	})
	assertAbsent(t, root, "cache/update/staging", "cache/update/replaced", archiveCache)
}

func TestApplyKeepsExecutableBitsFromTarArchives(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no executable permission bits")
	}
	root := installedRoot(t, "0.9.0")
	files := []releaseFile{{name: "raylea-server", payload: "new server", mode: 0o755}}
	checker, _ := releaseChecker(t, "tar.gz", releaseArchive(t, "tar.gz", "1.0.0", 0, files), nil)
	if _, err := checker.Apply(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "raylea-server"))
	if err != nil || info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("executable mode lost: %v %v", info, err)
	}
}

func TestApplyRejectsBrokenArchivesWithoutTouchingInstallation(t *testing.T) {
	corrupted := releaseArchive(t, "zip", "1.0.0", zip.Store, newRelease)
	reader, err := zip.NewReader(bytes.NewReader(corrupted), int64(len(corrupted)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range reader.File {
		if strings.HasSuffix(entry.Name, "raylea-server.exe") {
			offset, err := entry.DataOffset()
			if err != nil {
				t.Fatal(err)
			}
			corrupted[offset] ^= 1
		}
	}
	valid := releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease)
	for name, setup := range map[string]struct {
		archive []byte
		mutate  func(*Artifact)
	}{
		"size mismatch":       {archive: valid, mutate: func(artifact *Artifact) { artifact.ArchiveSizeBytes++ }},
		"crc mismatch":        {archive: corrupted},
		"build info mismatch": {archive: releaseArchive(t, "zip", "1.0.1", zip.Deflate, newRelease)},
	} {
		t.Run(name, func(t *testing.T) {
			root := installedRoot(t, "0.9.0")
			checker, _ := releaseChecker(t, "zip", setup.archive, setup.mutate)
			if _, err := checker.Apply(context.Background(), root); err == nil {
				t.Fatal("broken archive installed")
			}
			if InstalledVersion(root) != "0.9.0" {
				t.Fatalf("installed version changed to %s", InstalledVersion(root))
			}
			assertTree(t, root, map[string]string{"raylea-server.exe": "old server", "web/dist/index.html": "old index"})
			assertAbsent(t, root, archiveCache, archiveCache+".part")
		})
	}
}

func TestApplyFailureKeepsOldVersionAndRerunCompletes(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	blocker := filepath.Join(root, "templates", "help.menu", "template.json")
	if err := os.MkdirAll(filepath.Join(blocker, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	checker, downloads := releaseChecker(t, "zip", releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease), nil)
	if _, err := checker.Apply(context.Background(), root); err == nil {
		t.Fatal("apply succeeded over a conflicting directory")
	}
	if InstalledVersion(root) != "0.9.0" {
		t.Fatalf("interrupted update reported version %s", InstalledVersion(root))
	}
	if err := os.RemoveAll(blocker); err != nil {
		t.Fatal(err)
	}
	if _, err := checker.Apply(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if InstalledVersion(root) != "1.0.0" || *downloads != 1 {
		t.Fatalf("rerun installed %s after %d downloads", InstalledVersion(root), *downloads)
	}
	assertTree(t, root, map[string]string{"raylea-server.exe": "new server", "templates/help.menu/template.json": "template"})
}

func TestApplyWhenUpToDateChangesNothing(t *testing.T) {
	root := installedRoot(t, "1.0.0")
	checker, downloads := releaseChecker(t, "zip", releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease), nil)
	result, err := checker.Apply(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "up_to_date" || *downloads != 0 {
		t.Fatalf("result = %#v downloads = %d", result, *downloads)
	}
	assertTree(t, root, map[string]string{"raylea-server.exe": "old server"})
	assertAbsent(t, root, "cache")
}
