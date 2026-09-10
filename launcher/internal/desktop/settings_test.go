package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSettingsStoreCreatesNormalizedPortableSettings(t *testing.T) {
	root := createDevelopmentInstall(t)
	store := NewSettingsStore(filepath.Join(root, "launcher"))

	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !samePath(settings.InstallationRoot, root) {
		t.Fatalf("InstallationRoot = %q, want %q", settings.InstallationRoot, root)
	}
	if settings.CloseBehavior != closeAsk {
		t.Fatalf("CloseBehavior = %q, want %q", settings.CloseBehavior, closeAsk)
	}
	payload, err := os.ReadFile(filepath.Join(root, "data", "launcher.json"))
	if err != nil {
		t.Fatalf("read launcher settings: %v", err)
	}
	var persisted LauncherSettings
	if err := json.Unmarshal(payload, &persisted); err != nil {
		t.Fatalf("parse launcher settings: %v", err)
	}
	if !samePath(persisted.InstallationRoot, root) {
		t.Fatalf("persisted InstallationRoot = %q, want %q", persisted.InstallationRoot, root)
	}
}

func TestSettingsStorePreservesMalformedSettings(t *testing.T) {
	for _, original := range []string{"{not-json", "null", `{"closeBehavior":"unsupported"}`, `{"unknown":true}`, `{} {}`} {
		t.Run(original, func(t *testing.T) {
			root := createDevelopmentInstall(t)
			settingsPath := filepath.Join(root, "data", "launcher.json")
			writeTestFile(t, settingsPath, original)
			_, err := NewSettingsStore(root).Load()
			var boundary *BoundaryError
			if !errors.As(err, &boundary) || boundary.Code != "launcher.settings_invalid" {
				t.Fatalf("Load error = %v", err)
			}
			payload, readErr := os.ReadFile(settingsPath)
			if readErr != nil || string(payload) != original {
				t.Fatalf("damaged settings changed: %q %v", payload, readErr)
			}
		})
	}
}

func TestResolveLauncherSettingsPrefersBuiltServer(t *testing.T) {
	root := createDevelopmentInstall(t)
	name := "raylea-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	builtServer := filepath.Join(root, "server", "dist", name)
	writeTestFile(t, builtServer, "binary")

	resolved := ResolveLauncherSettings(LauncherSettings{InstallationRoot: root, CloseBehavior: closeAsk})
	if !samePath(resolved.ServerExecutablePath, builtServer) {
		t.Fatalf("ServerExecutablePath = %q, want %q", resolved.ServerExecutablePath, builtServer)
	}
	if !samePath(resolved.ConfigPath, filepath.Join(root, "config", "user.yaml")) {
		t.Fatalf("ConfigPath = %q", resolved.ConfigPath)
	}
}

func TestDiscoverBasePathPrefersExecutableInstallationOverWorkingDirectory(t *testing.T) {
	executableRoot := t.TempDir()
	writeTestFile(t, filepath.Join(executableRoot, "build_info.json"), `{ "version": "0.1.0", "artifact_id": "windows-x64-full" }`)
	writeTestFile(t, filepath.Join(executableRoot, ".deps", "manifest.json"), "{\"manifest_version\":4}\n")
	workingRoot := createDevelopmentInstall(t)
	executable := filepath.Join(executableRoot, "RayleaLauncher")

	got := discoverBasePath("", workingRoot, executable)
	if !samePath(got, executableRoot) {
		t.Fatalf("discoverBasePath() = %q, want executable root %q", got, executableRoot)
	}
}

func TestDiscoverBasePathUsesDevelopmentWorkingDirectoryForTemporaryExecutable(t *testing.T) {
	workingRoot := createDevelopmentInstall(t)
	temporaryExecutable := filepath.Join(t.TempDir(), "go-build", "RayleaLauncher")

	got := discoverBasePath("", workingRoot, temporaryExecutable)
	if !samePath(got, workingRoot) {
		t.Fatalf("discoverBasePath() = %q, want working root %q", got, workingRoot)
	}
}

func TestNormalizeSettingsRejectsUnsupportedCloseBehavior(t *testing.T) {
	_, err := normalizeSettings(LauncherSettings{InstallationRoot: t.TempDir(), CloseBehavior: "surprise"}, "")
	if err == nil {
		t.Fatal("normalizeSettings() accepted an unsupported close behavior")
	}
}

func TestSanitizeWebTargetPath(t *testing.T) {
	for _, valid := range []string{"", "/plugins", "/logs?scope=server#tail"} {
		if _, err := sanitizeWebTargetPath(valid); err != nil {
			t.Errorf("sanitizeWebTargetPath(%q) error = %v", valid, err)
		}
	}
	for _, invalid := range []string{"https://example.com", "//example.com", "relative", "/ok\nnext", "/../admin"} {
		if _, err := sanitizeWebTargetPath(invalid); err == nil {
			t.Errorf("sanitizeWebTargetPath(%q) unexpectedly succeeded", invalid)
		}
	}
}

func createDevelopmentInstall(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "server", "go.mod"), "module example/server\n")
	writeTestFile(t, filepath.Join(root, "launcher", "package.json"), "{}\n")
	return root
}

func writeTestFile(t *testing.T, destination, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(destination), err)
	}
	if err := os.WriteFile(destination, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", destination, err)
	}
}
