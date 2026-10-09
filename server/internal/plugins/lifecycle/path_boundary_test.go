package lifecycle

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestRuntimeRejectsUnsafePluginIDBeforePreparingDirectories(t *testing.T) {
	controller := newTestController(t, Deps{RepoRoot: t.TempDir()})
	for _, id := range []string{"../outside", `..\outside`, "/absolute", "C:relative", "..", "CON"} {
		if _, _, err := controller.buildStartInputs(t.Context(), id); !errors.Is(err, plugins.ErrInvalidPluginID) {
			t.Fatalf("id %q: %v", id, err)
		}
	}
}

func TestExtractZipRejectsPreexistingDirectorySymlink(t *testing.T) {
	tempRoot, outside := t.TempDir(), t.TempDir()
	extractRoot := filepath.Join(tempRoot, "unzipped")
	if err := os.Mkdir(extractRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(extractRoot, "plugin")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	archive := filepath.Join(t.TempDir(), "plugin.zip")
	writeZipEntries(t, archive, []zipTestEntry{{header: zip.FileHeader{Name: "plugin/payload.txt", Method: zip.Store}, content: "payload"}})
	if _, err := extractZipSource(t.Context(), archive, tempRoot); err == nil {
		t.Fatal("accepted symlink outside extraction root")
	}
	if _, err := os.Stat(filepath.Join(outside, "payload.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("wrote outside root: %v", err)
	}
}
