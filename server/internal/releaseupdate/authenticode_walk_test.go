package releaseupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuthenticodeWalkRequiresRootExecutablesAndChecksEveryPE(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"RayleaLauncher.exe",
		"raylea-server.exe",
		"raylea-updater.exe",
		"launcher/RayleaLauncher.exe",
		"launcher/libEGL.dll",
	} {
		writeFile(t, filepath.Join(root, name), []byte("PE"))
	}
	verified := 0
	required := 0
	if err := walkAuthenticodeFiles(root, func(_ string, requiredSigner bool) error {
		verified++
		if requiredSigner {
			required++
		}
		return nil
	}, new(int)); err != nil {
		t.Fatal(err)
	}
	if verified != 5 || required != 3 {
		t.Fatalf("verified=%d required=%d", verified, required)
	}
}

func TestAuthenticodeWalkDoesNotAcceptNestedRequiredExecutable(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"nested/RayleaLauncher.exe",
		"raylea-server.exe",
		"raylea-updater.exe",
	} {
		writeFile(t, filepath.Join(root, name), []byte("PE"))
	}
	count := 0
	err := walkAuthenticodeFiles(root, func(string, bool) error { return nil }, &count)
	if err == nil {
		t.Fatal("nested Launcher should not satisfy the required root executable")
	}
	if _, statErr := os.Stat(filepath.Join(root, "nested", "RayleaLauncher.exe")); statErr != nil {
		t.Fatal(statErr)
	}
}
