package pluginbuild

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBuildProducesAPlatformArtifactWithExactInventory(t *testing.T) {
	pluginDir := t.TempDir()
	writeTestFile(t, filepath.Join(pluginDir, "go.mod"), "module example.test/plugin\n\ngo 1.25.12\n")
	writeTestFile(t, filepath.Join(pluginDir, "main.go"), "package main\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(pluginDir, "LICENSE"), "test license\n")
	platform := testPlatform(t)
	manifest := map[string]any{
		"id": "test-plugin", "name": "Test", "version": "0.2.0", "manifest_version": "2",
		"plugin_protocol_version": "1", "runtime": "go", "entry": "bin/test-plugin",
		"platforms": []string{platform}, "license": "MIT",
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	writeTestFile(t, filepath.Join(pluginDir, "info.json"), string(manifestBytes)+"\n")

	result, err := Build(context.Background(), Config{PluginDir: pluginDir, OutputDir: filepath.Join(pluginDir, "out"), TargetPlatform: platform, KeepExpandedArtifact: true})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.ArchiveSHA256 == "" || result.ArtifactDir == "" {
		t.Fatalf("incomplete build result: %#v", result)
	}
	content, err := os.ReadFile(filepath.Join(result.ArtifactDir, "artifact.json"))
	if err != nil {
		t.Fatal(err)
	}
	var artifact Artifact
	if err := json.Unmarshal(content, &artifact); err != nil {
		t.Fatal(err)
	}
	roles := map[string]int{}
	for _, file := range artifact.Files {
		roles[file.Role]++
		if file.Path == "artifact.json" {
			t.Fatal("artifact.json must not inventory itself")
		}
	}
	if roles["backend"] != 1 || roles["manifest"] != 1 || roles["license"] != 1 || roles["notice"] != 1 || roles["sbom"] != 1 {
		t.Fatalf("unexpected role inventory: %#v", roles)
	}
	archive, err := zip.OpenReader(result.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if filepath.ToSlash(file.Name) == "test-plugin/main.go" || filepath.ToSlash(file.Name) == "test-plugin/go.mod" {
			t.Fatalf("source file leaked into artifact: %s", file.Name)
		}
		if filepath.ToSlash(file.Name) == "test-plugin/bin/test-plugin" && file.Mode().Perm() != 0o755 {
			t.Fatalf("backend archive mode = %o, want 755", file.Mode().Perm())
		}
	}
}

func TestBuildWorkspaceSBOMKeepsDeclaredSDKVersion(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "plugin")
	sdkDir := filepath.Join(root, "sdk")
	writeTestFile(t, filepath.Join(sdkDir, "go.mod"), "module example.test/sdk\n\ngo 1.25.12\n")
	writeTestFile(t, filepath.Join(sdkDir, "sdk.go"), "package sdk\nfunc Run() {}\n")
	writeTestFile(t, filepath.Join(pluginDir, "go.mod"), "module example.test/plugin\n\ngo 1.25.12\n\nrequire example.test/sdk v0.2.0\n")
	writeTestFile(t, filepath.Join(pluginDir, "main.go"), "package main\nimport \"example.test/sdk\"\nfunc main() { sdk.Run() }\n")
	writeTestFile(t, filepath.Join(pluginDir, "LICENSE"), "test license\n")
	platform := testPlatform(t)
	manifest := map[string]any{
		"id": "workspace-plugin", "name": "Workspace", "version": "0.2.0", "manifest_version": "2",
		"plugin_protocol_version": "1", "runtime": "go", "entry": "bin/workspace-plugin",
		"platforms": []string{platform}, "license": "MIT",
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	writeTestFile(t, filepath.Join(pluginDir, "info.json"), string(manifestBytes)+"\n")
	goWorkPath := filepath.Join(root, "go.work")
	writeTestFile(t, goWorkPath, fmt.Sprintf(
		"go 1.25.12\n\nuse (\n\t%q\n\t%q\n)\n\nreplace example.test/sdk v0.2.0 => %q\n",
		pluginDir, sdkDir, sdkDir,
	))
	t.Setenv("GOWORK", goWorkPath)
	t.Setenv("RAYLEA_PLUGIN_BUILD_USE_WORKSPACE", "1")

	result, err := Build(context.Background(), Config{
		PluginDir: pluginDir, OutputDir: filepath.Join(root, "out"), TargetPlatform: platform, KeepExpandedArtifact: true,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	payload, err := os.ReadFile(filepath.Join(result.ArtifactDir, "sbom.spdx.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sbom struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"versionInfo"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(payload, &sbom); err != nil {
		t.Fatal(err)
	}
	for _, item := range sbom.Packages {
		if item.Name == "example.test/sdk" && item.Version == "v0.2.0" {
			return
		}
	}
	t.Fatalf("workspace SDK version missing from SBOM: %#v", sbom.Packages)
}

func testPlatform(t *testing.T) string {
	t.Helper()
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "windows-x64"
	case "linux/amd64":
		return "linux-x64"
	case "darwin/arm64":
		return "macos-arm64"
	default:
		t.Skip("unsupported test host")
		return ""
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
