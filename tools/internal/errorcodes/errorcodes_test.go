package errorcodes

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/tools/internal/repo"
	"go.yaml.in/yaml/v3"
)

func TestCurrentCatalogsAreByteEquivalent(t *testing.T) {
	root := repo.Root()
	b, err := os.ReadFile(filepath.Join(root, "contracts/error-codes.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc Document
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	outputs, err := Generate(doc)
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range outputs {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n")), want) {
			t.Fatalf("catalog drift: %s", path)
		}
	}
}
func TestRejectInconsistentCatalog(t *testing.T) {
	status := 400
	for _, doc := range []Document{
		{Codes: map[string]Entry{"a.b": {Code: "a.c"}}},
		{Codes: map[string]Entry{"a.b": {Code: "a.b", HTTPStatus: &status}}},
		{Codes: map[string]Entry{"a.b": {Code: "a.b", Surfaces: []string{"http"}}}},
		{Codes: map[string]Entry{"a.b_c": {Code: "a.b_c"}, "a_b.c": {Code: "a_b.c"}}},
		{Codes: map[string]Entry{"a.b": {Code: "a.b"}}, Diagnostics: map[string]Diagnostic{"a.b": {Description: "duplicate"}}},
		{Diagnostics: map[string]Diagnostic{"a.b": {Description: " "}}},
	} {
		if _, err := Generate(doc); err == nil {
			t.Fatal("accepted inconsistent catalog")
		}
	}
}
func TestSyncOwnershipAndReadOnlyVerification(t *testing.T) {
	root := t.TempDir()
	outputs := map[string][]byte{"catalog/current.go": []byte("new\n")}
	dir := filepath.Join(root, "catalog")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{"current.go": "old", "stale.go": "// Code generated " + marker, "manual.go": "manual"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	stale, err := Sync(root, outputs, true)
	if err != nil || len(stale) != 2 {
		t.Fatalf("%v %v", stale, err)
	}
	current, err := os.ReadFile(filepath.Join(dir, "current.go"))
	if err != nil || string(current) != "old" {
		t.Fatal("verify modified catalog")
	}
	if _, err := Sync(root, outputs, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.go")); !os.IsNotExist(err) {
		t.Fatal("stale generated file survived")
	}
	if b, err := os.ReadFile(filepath.Join(dir, "manual.go")); err != nil || string(b) != "manual" {
		t.Fatal("unowned file modified")
	}
	if err := os.WriteFile(filepath.Join(dir, "current.go"), []byte("new\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stale, err = Sync(root, outputs, true)
	if err != nil || len(stale) != 0 {
		t.Fatalf("%v %v", stale, err)
	}
}
