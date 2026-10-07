package releaseupdate

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/filelock"
)

func response(request *http.Request, payload []byte) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(payload)), ContentLength: int64(len(payload)), Header: make(http.Header), Request: request}
}

func TestApplyPreparedDoesNotUseNetworkAndPinsVersion(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	checker, _ := releaseChecker(t, "zip", releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease), nil)
	result, _, err := checker.Download(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	checker.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { calls.Add(1); return nil, errors.New("offline") })}
	checker.DownloadClient = checker.HTTPClient
	if _, err = checker.ApplyPrepared(context.Background(), root, "staging-someone-else"); err == nil {
		t.Fatal("accepted another preparation")
	}
	if _, err = checker.ApplyPrepared(context.Background(), root, result.PreparedID); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || InstalledVersion(root) != "1.0.0" {
		t.Fatalf("network calls=%d installed=%s", calls.Load(), InstalledVersion(root))
	}
}

func TestMissingPreparationDoesNotCheckRemote(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	checker := NewChecker()
	checker.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("unexpected network")
		return nil, errors.New("offline")
	})}
	if _, err := checker.Apply(context.Background(), root); err == nil {
		t.Fatal("apply accepted missing preparation")
	}
}

func TestNoUpdateDiscardsAnOlderPendingSelection(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	checker, _ := releaseChecker(t, "zip", releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease), nil)
	if _, _, err := checker.Download(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	manifest := Manifest{Version: "0.9.0", ReleaseNotesRef: "https://example.com/release", Artifacts: []Artifact{{ArtifactID: ArtifactWindowsX64Full, FileName: "release.zip", ArchiveSizeBytes: 1, UpdateMode: "guided"}}}
	payload, _ := json.Marshal(manifest)
	checker.HTTPClient = manifestClient(payload)
	if result, _, err := checker.Download(context.Background(), root); err != nil || result.Status != "up_to_date" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := checker.Apply(context.Background(), root); err == nil {
		t.Fatal("applied previous channel's pending update")
	}
}

func TestExactVersionRejectsDowngrades(t *testing.T) {
	root := installedRoot(t, "1.1.0")
	checker, _ := releaseChecker(t, "zip", releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease), nil)
	checker.Settings.Version = "1.0.0"
	if _, err := checker.Check(context.Background(), root); err == nil {
		t.Fatal("accepted downgrade")
	}
}

func TestUnavailableBetaIndexDoesNotReportStableAsLatest(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	checker, _ := releaseChecker(t, "zip", releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease), nil)
	checker.Settings.Channel = "beta"
	if _, err := checker.Check(context.Background(), root); err == nil {
		t.Fatal("silently fell back to stable after release index failed")
	}
}

func TestUpdateOperationsRejectConcurrentPreparation(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	lock, err := filelock.Acquire(filepath.Join(root, "cache", "update", "operation.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, _, err := NewChecker().Download(context.Background(), root); !errors.Is(err, filelock.ErrLocked) {
		t.Fatalf("expected lock error, got %v", err)
	}
}

func TestNewestValidManifestWinsOverFastStaleOrHTMLSources(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	checker := NewChecker()
	checker.Settings = config.UpdateConfig{Channel: "stable", Mode: "auto", Proxies: []string{"https://stale.example", "https://html.example", "https://new.example"}}
	checker.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "github.com" {
			return nil, errors.New("blocked")
		}
		if request.URL.Host == "html.example" {
			return response(request, []byte("<html>200 OK</html>")), nil
		}
		version := "1.0.0"
		if request.URL.Host == "new.example" {
			version = "1.1.0"
		}
		payload, _ := json.Marshal(Manifest{Version: version, ReleaseNotesRef: "https://example.com/release", Artifacts: []Artifact{{ArtifactID: ArtifactWindowsX64Full, FileName: "release.zip", ArchiveSizeBytes: 1, UpdateMode: "guided"}}})
		return response(request, payload), nil
	})}
	result, err := checker.Check(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if result.AvailableVersion != "1.1.0" || !strings.HasPrefix(result.SourceURL, "https://new.example/") {
		t.Fatalf("result=%+v", result)
	}
	if len(result.Routes) != 4 || result.Routes[2].Available {
		t.Fatalf("routes=%+v", result.Routes)
	}
}

func TestReleaseChannelsFilterDraftsAndSortSemver(t *testing.T) {
	checker := NewChecker()
	checker.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return response(request, []byte(`[
	{"tag_name":"v1.9.0","html_url":"https://example.com/1.9"},
	{"tag_name":"v1.10.0-beta.2","prerelease":true,"html_url":"https://example.com/beta2"},
	{"tag_name":"v1.10.0-beta.10","prerelease":true,"html_url":"https://example.com/beta10"},
	{"tag_name":"v2.0.0","draft":true,"html_url":"https://example.com/draft"}]`)), nil
	})}
	stable, err := checker.Releases(context.Background())
	if err != nil || len(stable) != 1 || stable[0].Version != "1.9.0" {
		t.Fatalf("stable=%+v %v", stable, err)
	}
	checker.Settings.Channel = "beta"
	beta, err := checker.Releases(context.Background())
	if err != nil || len(beta) != 3 || beta[0].Version != "1.10.0-beta.10" {
		t.Fatalf("beta=%+v %v", beta, err)
	}
}

func TestDownloadFallsBackAfterBadArchive(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	archive := releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease)
	checker, _ := releaseChecker(t, "zip", archive, nil)
	checker.Settings = config.UpdateConfig{Channel: "stable", Mode: "auto", Mirrors: []string{"https://mirror.example"}}
	original := checker.DownloadClient
	var failures atomic.Int32
	checker.DownloadClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "example.com" {
			if request.Header.Get("Range") != "" {
				return response(request, archive), nil
			}
			failures.Add(1)
			return response(request, bytes.Repeat([]byte("x"), len(archive))), nil
		}
		if request.URL.Host == "mirror.example" {
			// An invalid probe sample ranks the mirror after the corrupt but valid-looking source.
			if request.Header.Get("Range") != "" {
				return response(request, []byte("not an archive")), nil
			}
			return response(request, archive), nil
		}
		return original.Transport.RoundTrip(request)
	})}
	result, _, err := checker.Download(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreparedID == "" || failures.Load() == 0 {
		t.Fatalf("result=%+v failures=%d", result, failures.Load())
	}
}

func TestLocalImportIsOfflineAndRejectsNonNewerVersion(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	source := filepath.Join(t.TempDir(), "release.zip")
	if err := os.WriteFile(source, releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease), 0600); err != nil {
		t.Fatal(err)
	}
	checker := NewChecker()
	checker.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Error("local import used network")
		return nil, errors.New("offline")
	})}
	if _, err := checker.Import(context.Background(), root, source); err != nil {
		t.Fatal(err)
	}
	if _, err := checker.Apply(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	if _, err := checker.Import(context.Background(), root, source); err == nil {
		t.Fatal("accepted a non-newer local version")
	}
}

func TestPreparationWriteFailureKeepsDownloadedArchiveWithoutRetry(t *testing.T) {
	root := installedRoot(t, "0.9.0")
	checker, downloads := releaseChecker(t, "zip", releaseArchive(t, "zip", "1.0.0", zip.Deflate, newRelease), nil)
	if err := os.MkdirAll(filepath.Join(root, "cache", "update", "prepared.json"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := checker.Download(context.Background(), root); err == nil {
		t.Fatal("preparation unexpectedly succeeded")
	}
	if *downloads != 1 {
		t.Fatalf("local write failure triggered %d downloads", *downloads)
	}
	if _, err := os.Stat(filepath.Join(root, archiveCache)); err != nil {
		t.Fatalf("downloaded archive was discarded: %v", err)
	}
}
