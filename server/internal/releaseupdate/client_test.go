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
	return BuildInfo{Version: version, GitCommit: "abcdef1", ArtifactID: ArtifactWindowsX64Full, BuiltAt: "2026-09-10T00:00:00Z", PluginManifestVersion: PluginManifestVersion, PluginUIBridgeVersion: PluginUIBridgeVersion}
}
func marshalBuildInfo(t *testing.T, info BuildInfo) []byte {
	t.Helper()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	return data
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
func TestCheckRejectsMalformedMetadataAndUnsafeReleaseLinks(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "build_info.json"), marshalBuildInfo(t, testBuildInfo("0.0.1")))
	fixture := releaseFixture(t)
	var manifest Manifest
	if err := json.Unmarshal(fixture, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.ReleaseNotesRef = "javascript:alert(1)"
	unsafeData, _ := json.Marshal(manifest)
	for _, data := range [][]byte{[]byte(`{`), unsafeData} {
		checker := NewChecker()
		checker.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
		})}
		if _, err := checker.Check(context.Background(), root); CodeOf(err) != CodeManifestInvalid {
			t.Fatalf("invalid metadata accepted: %v", err)
		}
	}
}
