package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
)

type cliRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn cliRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestVersionJSONReadsPackagedBuildInfo(t *testing.T) {
	root := t.TempDir()
	writeCLIJSON(t, filepath.Join(root, "build_info.json"), releaseupdate.BuildInfo{
		Version:               "1.2.3",
		GitCommit:             "0123456789abcdef0123456789abcdef01234567",
		ArtifactID:            "windows-x64-full",
		BuiltAt:               "2026-07-10T00:00:00Z",
		PluginManifestVersion: releaseupdate.PluginManifestVersion,
		PluginUIBridgeVersion: releaseupdate.PluginUIBridgeVersion,
	})
	var stdout bytes.Buffer
	code := Run(Command{
		Name:       "version",
		ConfigPath: filepath.Join(root, "config", "user.yaml"),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Args:       []string{"--json"},
		Stdout:     &stdout,
	})
	if code != 0 {
		t.Fatalf("version exit code = %d", code)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["version"] != "1.2.3" || result["artifact_id"] != "windows-x64-full" {
		t.Fatalf("unexpected version output: %#v", result)
	}
}

func TestUpdateCheckJSONUsesUnsignedMetadata(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	writeCLIJSON(t, filepath.Join(root, "build_info.json"), releaseupdate.BuildInfo{
		Version:               "1.0.0",
		GitCommit:             "0123456789abcdef0123456789abcdef01234567",
		ArtifactID:            "windows-x64-full",
		BuiltAt:               now.Add(-24 * time.Hour).Format(time.RFC3339),
		PluginManifestVersion: releaseupdate.PluginManifestVersion,
		PluginUIBridgeVersion: releaseupdate.PluginUIBridgeVersion,
	})
	manifestBytes, err := os.ReadFile("../../../fixtures/release-manifest/ok.release-manifest-minimal.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest = manifest["input"].(map[string]any)
	manifest["version"] = "1.1.0"
	manifestBytes, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: cliRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(request.URL.Path, releaseupdate.ManifestAssetName) {
			t.Fatalf("unexpected request: %s", request.URL)
		}
		payload := manifestBytes
		return &http.Response{StatusCode: http.StatusOK, ContentLength: int64(len(payload)), Body: io.NopCloser(bytes.NewReader(payload)), Header: make(http.Header), Request: request}, nil
	})}
	var stdout bytes.Buffer
	code := Run(Command{
		Name:             "update",
		ConfigPath:       filepath.Join(root, "config", "user.yaml"),
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		Args:             []string{"check", "--json"},
		Stdout:           &stdout,
		UpdateHTTPClient: client,
		Now:              func() time.Time { return now },
	})
	if code != 0 {
		t.Fatalf("update check exit code = %d", code)
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "update_available" || result["available_version"] != "1.1.0" || result["update_mode"] != "guided" {
		t.Fatalf("unexpected update output: %#v", result)
	}
}

func writeCLIJSON(t *testing.T, path string, value any) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	writeCLIBytes(t, path, payload)
}

func writeCLIBytes(t *testing.T, path string, payload []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}
