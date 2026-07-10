package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	modulePrefix           = "github.com/RayleaBot/RayleaBot/server/internal/"
	managementImportPath   = modulePrefix + "management"
	managementImportPrefix = managementImportPath + "/"
	appImportPrefix        = modulePrefix + "app"
)

var numberedTestPartNameRE = regexp.MustCompile(`_part[0-9]+_`)

func TestManagementPackagesDoNotLeakIntoDomainPackages(t *testing.T) {
	serverRoot := testServerRoot(t)
	internalRoot := filepath.Join(serverRoot, "internal")

	walkGoFiles(t, internalRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || pathWithin(path, filepath.Join(internalRoot, "app")) || pathWithin(path, filepath.Join(internalRoot, "management")) {
			return
		}

		for _, importPath := range fileImports(t, serverRoot, path) {
			if importPath == managementImportPath || strings.HasPrefix(importPath, managementImportPrefix) {
				t.Errorf("%s imports management package %s", relPath(t, serverRoot, path), importPath)
			}
		}
	})
}

func TestRenderImplementationPackagesStayBehindServiceBoundary(t *testing.T) {
	serverRoot := testServerRoot(t)
	internalRoot := filepath.Join(serverRoot, "internal")
	renderRoot := filepath.Join(internalRoot, "render")
	protectedPrefixes := []string{
		modulePrefix + "render/repository",
	}

	walkGoFiles(t, internalRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || pathWithin(path, renderRoot) {
			return
		}
		for _, importPath := range fileImports(t, serverRoot, path) {
			for _, protectedPrefix := range protectedPrefixes {
				if importPath == protectedPrefix || strings.HasPrefix(importPath, protectedPrefix+"/") {
					t.Errorf("%s imports render implementation package %s", relPath(t, serverRoot, path), importPath)
				}
			}
		}
	})
}

func TestInternalTreeHasNoEmptyDirectories(t *testing.T) {
	serverRoot := testServerRoot(t)
	internalRoot := filepath.Join(serverRoot, "internal")

	if err := filepath.WalkDir(internalRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		switch entry.Name() {
		case ".git", "dist", ".gocache":
			return filepath.SkipDir
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			t.Errorf("%s is an empty directory", relPath(t, serverRoot, path))
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", internalRoot, err)
	}
}

// TestDomainPackagesDoNotImportApp forbids domain packages from importing the
// composition root. Only the entry/assembly layer (internal/app) and the
// server/tests tree may depend on internal/app.
func TestDomainPackagesDoNotImportApp(t *testing.T) {
	serverRoot := testServerRoot(t)
	internalRoot := filepath.Join(serverRoot, "internal")

	exempt := []string{
		filepath.Join(internalRoot, "app"),
	}

	walkGoFiles(t, internalRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		for _, root := range exempt {
			if pathWithin(path, root) {
				return
			}
		}
		for _, importPath := range fileImports(t, serverRoot, path) {
			if importPath == appImportPrefix || strings.HasPrefix(importPath, appImportPrefix+"/") {
				t.Errorf("%s imports composition root %s", relPath(t, serverRoot, path), importPath)
			}
		}
	})
}

func TestTestFilesUseScenarioNames(t *testing.T) {
	serverRoot := testServerRoot(t)

	walkGoFiles(t, serverRoot, func(path string) {
		if !strings.HasSuffix(path, "_test.go") {
			return
		}
		name := filepath.Base(path)
		if numberedTestPartNameRE.MatchString(name) {
			t.Errorf("%s uses numbered part naming; use a behavior or scenario name", relPath(t, serverRoot, path))
		}
	})
}

func fileImports(t *testing.T, serverRoot, path string) []string {
	t.Helper()

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s imports: %v", relPath(t, serverRoot, path), err)
	}
	imports := make([]string, 0, len(parsed.Imports))
	for _, imported := range parsed.Imports {
		imports = append(imports, strings.Trim(imported.Path.Value, `"`))
	}
	return imports
}

func testServerRoot(t *testing.T) string {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return filepath.Clean(filepath.Join(cwd, "..", ".."))
}

func walkGoFiles(t *testing.T, root string, visit func(string)) {
	t.Helper()

	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "dist", ".gocache":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			visit(path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

func relPath(t *testing.T, root, path string) string {
	t.Helper()

	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("relpath %s from %s: %v", path, root, err)
	}
	return filepath.ToSlash(rel)
}

func isGeneratedGoFile(path string) bool {
	name := filepath.Base(path)
	if strings.HasSuffix(name, "_gen.go") || strings.HasSuffix(name, ".pb.go") || strings.Contains(name, ".generated.") {
		return true
	}
	normalized := filepath.ToSlash(path)
	if strings.Contains(normalized, "/internal/sqlcgen/") {
		return true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	limit := len(data)
	if limit > 512 {
		limit = 512
	}
	return strings.Contains(string(data[:limit]), "Code generated")
}
