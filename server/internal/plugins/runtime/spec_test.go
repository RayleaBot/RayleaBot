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
	if spec.WorkDir != root || spec.EntryPath != spec.Command || spec.Runtime != "go" {
		t.Fatalf("unexpected runtime spec: %#v", spec)
	}
	if spec.InitTimeout != 2*time.Second || spec.EventTimeout != 3*time.Second || spec.ShutdownGrace != 4*time.Second || spec.EffectiveConcurrency != 2 {
		t.Fatalf("runtime limits were not projected: %#v", spec)
	}
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

func TestBuildSpecRejectsLegacyRuntimeAndInvalidCatalogEntry(t *testing.T) {
	_, snapshot := runtimeTestArtifact(t)
	snapshot.Runtime = "python"
	_, err := BuildSpec(snapshot, "", minimalRuntimeConfig())
	assertBuildSpecErrorCode(t, err, codePlatformInvalidRequest)

	_, snapshot = runtimeTestArtifact(t)
	snapshot.Valid = false
	_, err = BuildSpec(snapshot, "", minimalRuntimeConfig())
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
		"id": "runtime-test", "name": "Runtime test", "version": "0.2.0", "manifest_version": "2",
		"plugin_protocol_version": "1", "runtime": "go", "entry": logicalEntry, "platforms": []string{platform}, "license": "MIT",
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(filepath.Join(root, "info.json"), manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	manifestHash := sha256.Sum256(manifestBytes)
	document := map[string]any{
		"artifact_version": "1", "plugin_id": "runtime-test", "plugin_version": "0.2.0", "target_platform": platform,
		"manifest_sha256": hex.EncodeToString(manifestHash[:]),
		"files": []any{
			runtimeArtifactFile(t, root, "info.json", "manifest"),
			runtimeArtifactFile(t, root, backendRelative, "backend"),
		},
	}
	artifactBytes, _ := json.MarshalIndent(document, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "artifact.json"), append(artifactBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, plugins.Snapshot{
		PluginID: "runtime-test", Name: "Runtime test", Valid: true, Runtime: "go", Entry: logicalEntry,
		ManifestPath: filepath.Join(root, "info.json"), PackageRootPath: root, Concurrency: 4,
	}
}

func runtimeArtifactFile(t *testing.T, root, relative, role string) map[string]any {
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
	return map[string]any{"path": filepath.ToSlash(relative), "role": role, "size": info.Size(), "sha256": hex.EncodeToString(digest[:])}
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
