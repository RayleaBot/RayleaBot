package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	// maxProductionFileLines is a loose safety ceiling that only catches
	// pathologically large files. It is NOT an architectural boundary: package
	// responsibility is expressed through dependency direction and type
	// cohesion (see TestCompositionRootLayering / TestDomainPackagesDoNotImportApp),
	// not file or line counts.
	maxProductionFileLines = 1500

	modulePrefix           = "github.com/RayleaBot/RayleaBot/server/internal/"
	managementImportPrefix = modulePrefix + "management/"
	appImportPrefix        = modulePrefix + "app"
)

func TestManagementPackagesDoNotLeakIntoDomainPackages(t *testing.T) {
	serverRoot := testServerRoot(t)
	internalRoot := filepath.Join(serverRoot, "internal")

	walkGoFiles(t, internalRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || pathWithin(path, filepath.Join(internalRoot, "app")) || pathWithin(path, filepath.Join(internalRoot, "management")) {
			return
		}

		for _, importPath := range fileImports(t, serverRoot, path) {
			if strings.HasPrefix(importPath, managementImportPrefix) {
				t.Errorf("%s imports management package %s", relPath(t, serverRoot, path), importPath)
			}
		}
	})
}

// TestCompositionRootLayering enforces the one-directional assembly order of
// the app composition root: platform -> pluginstack -> servicegraph -> httpwire.
// A lower layer must never import a higher one, and only internal/app itself may
// reach across all four sub-packages.
func TestCompositionRootLayering(t *testing.T) {
	serverRoot := testServerRoot(t)
	appRoot := filepath.Join(serverRoot, "internal", "app")

	const (
		platform    = appImportPrefix + "/platform"
		pluginstack = appImportPrefix + "/pluginstack"
		servicegraph = appImportPrefix + "/servicegraph"
		httpwire    = appImportPrefix + "/httpwire"
	)
	// forbidden maps a sub-package directory to the higher layers it must not import.
	forbidden := map[string][]string{
		"platform":     {pluginstack, servicegraph, httpwire},
		"pluginstack":  {servicegraph, httpwire},
		"servicegraph": {httpwire},
	}

	for subDir, higherLayers := range forbidden {
		dir := filepath.Join(appRoot, subDir)
		walkGoFiles(t, dir, func(path string) {
			if strings.HasSuffix(path, "_test.go") {
				return
			}
			for _, importPath := range fileImports(t, serverRoot, path) {
				for _, higher := range higherLayers {
					if importPath == higher || strings.HasPrefix(importPath, higher+"/") {
						t.Errorf("%s imports higher composition layer %s", relPath(t, serverRoot, path), importPath)
					}
				}
			}
		})
	}
}

// TestDomainPackagesDoNotImportApp forbids domain packages from importing the
// composition root. Only the entry/assembly layer (internal/app,
// internal/bootstrap), test harnesses (internal/testapp) and the server/tests
// tree may depend on internal/app or internal/app/httpwire.
func TestDomainPackagesDoNotImportApp(t *testing.T) {
	serverRoot := testServerRoot(t)
	internalRoot := filepath.Join(serverRoot, "internal")

	exempt := []string{
		filepath.Join(internalRoot, "app"),
		filepath.Join(internalRoot, "bootstrap"),
		filepath.Join(internalRoot, "testapp"),
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

func TestProductionFilesStayReadable(t *testing.T) {
	serverRoot := testServerRoot(t)

	walkGoFiles(t, serverRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || isGeneratedGoFile(path) {
			return
		}
		lineCount, err := countLines(path)
		if err != nil {
			t.Fatalf("count %s lines: %v", relPath(t, serverRoot, path), err)
		}
		if lineCount > maxProductionFileLines {
			t.Errorf("%s has %d lines, want <= %d", relPath(t, serverRoot, path), lineCount, maxProductionFileLines)
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
	return strings.HasSuffix(name, "_gen.go") || strings.Contains(name, ".generated.")
}

func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	count := strings.Count(string(data), "\n")
	if data[len(data)-1] != '\n' {
		count++
	}
	return count, nil
}
