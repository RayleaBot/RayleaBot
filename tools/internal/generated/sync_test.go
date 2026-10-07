package generated

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestOwnedOutputs(t *testing.T) {
	root := t.TempDir()
	outputs := map[string][]byte{"wire/models.generated.go": []byte("// fixture marker\npackage wire\n")}
	dirs := []string{"wire"}
	check := func(want []string) {
		t.Helper()
		got, e := Sync(root, outputs, "fixture marker", dirs, true)
		if e != nil || !slices.Equal(got, want) {
			t.Fatalf("got %v %v, want %v", got, e, want)
		}
	}
	check([]string{"wire/models.generated.go"})
	if _, e := Sync(root, outputs, "fixture marker", dirs, false); e != nil {
		t.Fatal(e)
	}
	check(nil)
	for p, s := range map[string]string{"wire/models.generated.go": "changed", "wire/old.generated.go": "// fixture marker", "wire/manual.go": "package wire", "wire/old.schema.json": "{}", "wire/old.generated.json": "{}"} {
		if e := os.WriteFile(filepath.Join(root, p), []byte(s), 0600); e != nil {
			t.Fatal(e)
		}
	}
	check([]string{"wire/models.generated.go", "wire/old.generated.go", "wire/old.generated.json", "wire/old.schema.json"})
	if _, e := Sync(root, outputs, "fixture marker", dirs, false); e != nil {
		t.Fatal(e)
	}
	check(nil)
	if _, e := os.Stat(filepath.Join(root, "wire/manual.go")); e != nil {
		t.Fatal("unowned file was removed", e)
	}
}
