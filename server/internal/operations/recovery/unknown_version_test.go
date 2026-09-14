package recovery

import "testing"

func TestBackupManifestKeepsUnknownBuildVersionExplicit(t *testing.T) {
	root := t.TempDir()
	manifest := BuildBackupManifest(root, "online")
	manifest.Directories = []BackupManifestDirectory{Directory("config/user.yaml", "config")}
	if err := ValidateBackupManifest(manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.CoreVersion != "unknown" {
		t.Fatal(manifest.CoreVersion)
	}
}
