package recovery

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestBackupAndRestoreKeepUnknownBuildVersionExplicit(t *testing.T) {
	root := t.TempDir()
	manifest := BuildBackupManifest(root, "online")
	manifest.Directories = []BackupManifestDirectory{Directory("config/user.yaml", "config")}
	if err := ValidateBackupManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.CoreVersion != "unknown" {
		t.Fatal(manifest.CoreVersion)
	}
	manifest.CoreVersion = "1.2.3"
	result := EvaluateRestore(manifest, root)
	if result.Operation != "restore" || result.TargetCoreVersion != "unknown" {
		t.Fatalf("unknown build classified as upgrade or rollback: %+v", result)
	}
	code, _ := pluginCompatibilityIssue(plugins.Snapshot{
		ManifestVersion: PluginManifestVersion, ArtifactVersion: PluginArtifactVersion,
		MinCoreVersion: "0.0.0",
	}, "unknown")
	if code != "plugin.min_core_version" {
		t.Fatalf("unknown build accepted minimum version: %q", code)
	}
}
