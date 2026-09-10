package releaseupdate

import (
	"path/filepath"
	"testing"
)

func TestInstalledVersionRequiresValidBuildMetadata(t *testing.T) {
	root := t.TempDir()
	if got := InstalledVersion(root); got != "unknown" {
		t.Fatalf("missing metadata version = %q", got)
	}
	path := filepath.Join(root, "build_info.json")
	for _, payload := range []string{`{`, `{}`, `{"version":"0.4.0"}`} {
		writeFile(t, path, []byte(payload))
		if got := InstalledVersion(root); got != "unknown" {
			t.Fatalf("invalid metadata %s produced version %q", payload, got)
		}
	}
	writeFile(t, path, marshalBuildInfo(t, testBuildInfo("1.2.3", "windows-x64-full")))
	if got := InstalledVersion(root); got != "1.2.3" {
		t.Fatalf("release version = %q", got)
	}
}
