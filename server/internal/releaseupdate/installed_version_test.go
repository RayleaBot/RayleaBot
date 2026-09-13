package releaseupdate

import (
	"path/filepath"
	"testing"
)

func TestInstalledVersionReadsBuildMetadataVersion(t *testing.T) {
	root := t.TempDir()
	if got := InstalledVersion(root); got != "unknown" {
		t.Fatalf("missing metadata version = %q", got)
	}
	path := filepath.Join(root, "build_info.json")
	for _, payload := range []string{`{`, `{}`, `{"version":"latest"}`} {
		writeFile(t, path, []byte(payload))
		if got := InstalledVersion(root); got != "unknown" {
			t.Fatalf("invalid metadata %s produced version %q", payload, got)
		}
	}
	writeFile(t, path, []byte(`{"version":"0.4.0","future_field":true}`))
	if got := InstalledVersion(root); got != "0.4.0" {
		t.Fatalf("release version = %q", got)
	}
}
