package cli

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/releaseupdate"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

type failingCommandWriter struct{}

func (failingCommandWriter) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestVersionReturnsFailureWhenJSONOutputCannotBeWritten(t *testing.T) {
	root := t.TempDir()
	writeCLIJSON(t, filepath.Join(root, "build_info.json"), releaseupdate.BuildInfo{
		Version: "1.2.3", GitCommit: "0123456789abcdef0123456789abcdef01234567", ArtifactID: "windows-x64-full", BuiltAt: "2026-07-10T00:00:00Z",
		UpdateProtocolVersion: releaseupdate.ProtocolVersion, PluginManifestVersion: releaseupdate.PluginManifestVersion, PluginUIBridgeVersion: releaseupdate.PluginUIBridgeVersion,
	})
	var logs bytes.Buffer
	code := Run(Command{Name: "version", ConfigPath: filepath.Join(root, "config", "user.yaml"), Logger: slog.New(slog.NewJSONHandler(&logs, nil)), Args: []string{"--json"}, Stdout: failingCommandWriter{}})
	if code != 1 || !bytes.Contains(logs.Bytes(), []byte(`"level":"ERROR"`)) {
		t.Fatalf("writer failure not observable: exit=%d logs=%s", code, logs.Bytes())
	}
}

func TestDevelopmentSyncOutputFailureReportsCommittedInstallation(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config", "user.yaml")
	writeFile(t, configPath, "database:\n  path: data/rayleabot.db\n")
	artifact := testutil.WriteGoPluginArtifact(t, filepath.Join(t.TempDir(), "artifact"), "output.fixture", "0.4.0")
	var logs bytes.Buffer
	code := Run(Command{Name: "plugin", ConfigPath: configPath, Logger: slog.New(slog.NewJSONHandler(&logs, nil)), Args: []string{"dev-sync", "--artifact", artifact, "--source", t.TempDir()}, Stdout: failingCommandWriter{}})
	if code != 1 || !bytes.Contains(logs.Bytes(), []byte(`"committed":true`)) {
		t.Fatalf("committed output failure not observable: exit=%d logs=%s", code, logs.Bytes())
	}
	if _, err := os.Stat(filepath.Join(root, "plugins", "installed", "output.fixture", "info.json")); err != nil {
		t.Fatalf("completed installation disappeared after output failure: %v", err)
	}
}
