package releaseupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func releaseFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../../fixtures/release-manifest/ok.release-manifest-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Input json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.Input
}

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func testBuildInfo(version string) BuildInfo {
	return BuildInfo{Version: version, GitCommit: "abcdef1", ArtifactID: ArtifactWindowsX64Full}
}
func marshalBuildInfo(t *testing.T, info BuildInfo) []byte {
	t.Helper()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func manifestClient(data []byte) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
	})}
}
func TestCheckReadsUnsignedMetadataWithoutDownloadingOrPersisting(t *testing.T) {
	for _, version := range []string{"0.0.1", "99.0.0"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "build_info.json"), marshalBuildInfo(t, testBuildInfo(version)))
			data := releaseFixture(t)
			var manifest Manifest
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			checker := NewChecker()
			checker.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() != checker.ManifestURL {
					t.Fatalf("unexpected download: %s", r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
			})}
			result, err := checker.Check(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			want := "update_available"
			if version == "99.0.0" {
				want = "up_to_date"
			}
			if result.Status != want || result.ReleasePageURL != manifest.ReleaseNotesRef || result.UpdateMode != "guided" {
				t.Fatalf("unexpected result: %#v", result)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("check wrote installation state: %v", entries)
			}
		})
	}
}

// A newer release may add fields, change plugin contract versions or list
// artifacts this installation does not know; the check must still find it.
func TestCheckIgnoresUnknownAndUnusedReleaseFields(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "build_info.json"), []byte(`{"version":"0.9.0","artifact_id":"windows-x64-full","plugin_manifest_version":"99","future_field":true}`))
	data := []byte(`{
		"version": "1.0.0",
		"release_notes_ref": "https://example.com/releases/v1.0.0",
		"plugin_manifest_version": "99",
		"plugin_ui_bridge_version": "99",
		"future_field": {"nested": true},
		"artifacts": [
			{"artifact_id": "future-artifact", "file_name": "future.pkg", "archive_size_bytes": 1, "update_mode": "future"},
			{"artifact_id": "windows-x64-full", "file_name": "RayleaBot-v1.0.0-windows-x64-full.zip", "archive_size_bytes": 10, "update_mode": "guided", "future_field": 1}
		]
	}`)
	checker := NewChecker()
	checker.HTTPClient = manifestClient(data)
	result, err := checker.Check(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "update_available" || result.AvailableVersion != "1.0.0" || result.Artifact.FileName != "RayleaBot-v1.0.0-windows-x64-full.zip" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCheckRejectsMalformedMetadataAndUnsafeValues(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "build_info.json"), marshalBuildInfo(t, testBuildInfo("0.0.1")))
	fixture := releaseFixture(t)
	var unsafeLink Manifest
	if err := json.Unmarshal(fixture, &unsafeLink); err != nil {
		t.Fatal(err)
	}
	unsafeLink.ReleaseNotesRef = "javascript:alert(1)"
	unsafeLinkData, _ := json.Marshal(unsafeLink)
	var unsafeFile Manifest
	if err := json.Unmarshal(fixture, &unsafeFile); err != nil {
		t.Fatal(err)
	}
	unsafeFile.Artifacts[0].FileName = "../RayleaBot.zip"
	unsafeFileData, _ := json.Marshal(unsafeFile)
	for _, data := range [][]byte{[]byte(`{`), unsafeLinkData, unsafeFileData} {
		checker := NewChecker()
		checker.HTTPClient = manifestClient(data)
		if _, err := checker.Check(context.Background(), root); CodeOf(err) != CodeManifestInvalid {
			t.Fatalf("invalid metadata accepted: %v", err)
		}
	}
}
