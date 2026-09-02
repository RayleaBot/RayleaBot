package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPackResolvesRelativeInputsAgainstPluginRoot(t *testing.T) {
	pluginRoot := t.TempDir()
	outside := t.TempDir()
	platform := currentTestPlatform(t)
	binaryName := "native-plugin"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryRelative := filepath.ToSlash(filepath.Join("dist", "native", platform, binaryName))
	binaryPath := filepath.Join(pluginRoot, filepath.FromSlash(binaryRelative))
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binaryPath, payload, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"id": "native-plugin", "name": "Native Plugin", "version": "0.4.0",
		"manifest_version": "3", "min_core_version": "0.4.0", "license": "MIT",
	}
	manifestBytes, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginRoot, "info.json"), manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "LICENSE"), []byte("test license\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "extra.txt"), []byte("asset\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(outside); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDirectory) })

	if err := pack([]string{
		"--plugin", pluginRoot,
		"--binary", binaryRelative,
		"--target", platform,
		"--out", "artifacts",
		"--include", "extra.txt=extras/extra.txt",
	}); err != nil {
		t.Fatalf("pack() error = %v", err)
	}
	expanded := filepath.Join(pluginRoot, "artifacts", platform, "native-plugin")
	if _, err := os.Stat(filepath.Join(expanded, "artifact.json")); err != nil {
		t.Fatalf("relative output was not resolved below plugin root: %v", err)
	}
	if asset, err := os.ReadFile(filepath.Join(expanded, "extras", "extra.txt")); err != nil || string(asset) != "asset\n" {
		t.Fatalf("relative include was not resolved below plugin root: %q, %v", asset, err)
	}
	if _, err := os.Stat(filepath.Join(outside, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("pack wrote relative output below the caller CWD: %v", err)
	}
}

func currentTestPlatform(t *testing.T) string {
	t.Helper()
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		return "windows-x64"
	case "linux/amd64":
		return "linux-x64"
	case "darwin/arm64":
		return "macos-arm64"
	default:
		t.Skip("unsupported test platform")
		return ""
	}
}
