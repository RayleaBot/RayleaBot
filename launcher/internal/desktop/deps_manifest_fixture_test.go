package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagedManifestSharedFixtures(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "..", "fixtures", "deps-manifest", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("fixture discovery: %v, %d files", err, len(paths))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			payload, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			err = validateManifestJSON(payload)
			valid := !strings.HasPrefix(filepath.Base(path), "invalid.")
			if (err == nil) != valid {
				t.Fatalf("valid=%t, validation=%v", valid, err)
			}
		})
	}
}
