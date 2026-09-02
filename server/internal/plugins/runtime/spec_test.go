package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/artifact"
)

func TestBuildSpecUsesVerifiedGoExecutableDirectly(t *testing.T) {
	root, snapshot := runtimeTestArtifact(t)
	cfg := minimalRuntimeConfig()
	spec, err := BuildSpec(snapshot, "", cfg)
	if err != nil {
		t.Fatalf("BuildSpec() error = %v", err)
	}
	if spec.Command != filepath.Join(root, filepath.FromSlash(runtimeBackendRelative(t))) {
		t.Fatalf("command = %q", spec.Command)
	}
	if len(spec.Args) != 0 || len(spec.Env) != 0 {
		t.Fatalf("prebuilt Go plugins must start without interpreter args/env: args=%#v env=%#v", spec.Args, spec.Env)
	}
	if spec.WorkDir != root || spec.EntryPath != spec.Command || spec.Runtime != "native" {
		t.Fatalf("unexpected runtime spec: %#v", spec)
	}
	if spec.InitTimeout != 2*time.Second || spec.EventTimeout != 3*time.Second || spec.ShutdownGrace != 4*time.Second || spec.EffectiveConcurrency != 2 {
		t.Fatalf("runtime limits were not projected: %#v", spec)
	}
	if spec.IPCPendingActionsMax != 7 || spec.IPCActionBurstCount != 11 || spec.IPCActionBurstWindow != 2*time.Second || spec.IPCMessageMaxBytes != 4096 {
		t.Fatalf("IPC limits were not projected: %#v", spec)
	}
}

func TestManagedRuntimeEnvironmentExposesPreparedFFmpegTools(t *testing.T) {
	repoRoot := t.TempDir()
	platform := artifactPlatformForTest(t)
	resource := map[string]any{
		"id": "ffmpeg-test", "kind": "ffmpeg", "version": "9.0.1", "platform": platform,
		"sources":        []any{map[string]any{"url": "https://example.invalid/ffmpeg.zip", "kind": "upstream"}},
		"sha256":         "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"archive_format": "zip",
		"entrypoints": map[string]any{
			"ffmpeg": []string{"bin/ffmpeg"}, "ffprobe": []string{"bin/ffprobe"},
		},
	}
	manifest := map[string]any{"manifest_version": 5, "resources": []any{resource}}
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(repoRoot, ".deps", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	storeRoot := filepath.Join(repoRoot, ".deps", "store", "ffmpeg-test", "9.0.1", "bin")
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ffmpeg", "ffprobe"} {
		if err := os.WriteFile(filepath.Join(storeRoot, name), []byte("fixture"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got := managedRuntimeEnvironment(repoRoot)
	want := []string{
		"RAYLEABOT_FFMPEG_PATH=" + filepath.Join(storeRoot, "ffmpeg"),
		"RAYLEABOT_FFPROBE_PATH=" + filepath.Join(storeRoot, "ffprobe"),
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("managed runtime environment = %#v, want %#v", got, want)
	}
}

func artifactPlatformForTest(t *testing.T) string {
	t.Helper()
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	return platform
}

func TestBuildSpecRejectsTamperedArtifact(t *testing.T) {
	_, snapshot := runtimeTestArtifact(t)
	file, err := os.OpenFile(snapshot.PackageRootPath+string(filepath.Separator)+filepath.FromSlash(runtimeBackendRelative(t)), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("tampered"))
	_ = file.Close()
	_, err = BuildSpec(snapshot, "", minimalRuntimeConfig())
	assertBuildSpecErrorCode(t, err, codePluginArtifactInvalid)
}

func TestBuildSpecRejectsInvalidCatalogEntry(t *testing.T) {
	_, snapshot := runtimeTestArtifact(t)
	snapshot.Valid = false
	_, err := BuildSpec(snapshot, "", minimalRuntimeConfig())
	assertBuildSpecErrorCode(t, err, codePlatformInvalidRequest)
}

func runtimeTestArtifact(t *testing.T) (string, plugins.Snapshot) {
	t.Helper()
	root := t.TempDir()
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	logicalEntry := "bin/runtime-test"
	backendRelative := logicalEntry
	if platform == "windows-x64" {
		backendRelative += ".exe"
	}
	backend := filepath.Join(root, filepath.FromSlash(backendRelative))
	if err := os.MkdirAll(filepath.Dir(backend), 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyRuntimeTestFile(t, executable, backend)
	manifest := map[string]any{
		"id": "runtime-test", "name": "Runtime test", "version": "0.4.0", "manifest_version": "3",
		"license": "MIT", "min_core_version": "0.4.0",
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(filepath.Join(root, "info.json"), manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	document := map[string]any{
		"artifact_version": "2", "target_platform": platform, "entry": backendRelative,
		"files": []any{
			runtimeArtifactFile(t, root, "info.json"),
			runtimeArtifactFile(t, root, backendRelative),
		},
	}
	artifactBytes, _ := json.MarshalIndent(document, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "artifact.json"), append(artifactBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, plugins.Snapshot{
		PluginID: "runtime-test", Name: "Runtime test", Valid: true,
		ManifestPath: filepath.Join(root, "info.json"), PackageRootPath: root, Concurrency: 4,
	}
}

func runtimeArtifactFile(t *testing.T, root, relative string) map[string]any {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	return map[string]any{"path": filepath.ToSlash(relative), "size": info.Size(), "sha256": hex.EncodeToString(digest[:])}
}

func runtimeBackendRelative(t *testing.T) string {
	t.Helper()
	platform, err := artifact.CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	if platform == "windows-x64" {
		return "bin/runtime-test.exe"
	}
	return "bin/runtime-test"
}

func copyRuntimeTestFile(t *testing.T, source, destination string) {
	t.Helper()
	input, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(output, input); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func minimalRuntimeConfig() config.RuntimeConfig {
	return config.RuntimeConfig{
		PluginInitTimeoutSeconds:    2,
		PluginInitMaxTotalSeconds:   5,
		PluginEventTimeoutSeconds:   3,
		ShutdownGraceSeconds:        4,
		MaxConcurrentTasksPerPlugin: 2,
		IPCPendingActionsMax:        7,
		IPCActionBurstLimit:         "11/2s",
		IPCMessageMaxBytes:          4096,
	}
}

func assertBuildSpecErrorCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected runtime error %q", want)
	}
	runtimeErr, ok := err.(*Error)
	if !ok || runtimeErr.Code != want {
		t.Fatalf("error = %T %v, want code %q", err, err, want)
	}
}
