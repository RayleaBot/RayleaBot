package architecture_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNeutralOutboundDoesNotImportProtocolImplementations(t *testing.T) {
	root := testServerRoot(t)
	for _, name := range []string{"bot/chatevent", "bot/pipeline/outbound", "bot/pipeline/dispatch"} {
		files := 0
		walkGoFiles(t, filepath.Join(root, "internal", filepath.FromSlash(name)), func(path string) {
			if strings.HasSuffix(path, "_test.go") {
				return
			}
			files++
			for _, imported := range fileImports(t, root, path) {
				if imported == modulePrefix+"bot/adapters/onebot11" || imported == modulePrefix+"bot/adapters/qqofficial" {
					t.Errorf("%s imports protocol implementation %s", relPath(t, root, path), imported)
				}
			}
		})
		if files == 0 {
			t.Fatalf("boundary scanned no production files: %s", name)
		}
	}
}
