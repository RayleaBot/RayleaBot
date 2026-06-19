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
	maxProductionFilesPerDir = 19
	maxTestFilesPerDir       = 20
	maxProductionFileLines   = 600
	managementImportPrefix   = "github.com/RayleaBot/RayleaBot/server/internal/management/"
)

func TestServerPackageFileCountsStayBounded(t *testing.T) {
	serverRoot := testServerRoot(t)
	counts := map[string]struct {
		production int
		tests      int
	}{}

	walkGoFiles(t, serverRoot, func(path string) {
		dir := filepath.Dir(path)
		entry := counts[dir]
		if strings.HasSuffix(path, "_test.go") {
			entry.tests++
		} else {
			entry.production++
		}
		counts[dir] = entry
	})

	for dir, count := range counts {
		if count.production > maxProductionFilesPerDir {
			t.Errorf("%s has %d production Go files, want <= %d", relPath(t, serverRoot, dir), count.production, maxProductionFilesPerDir)
		}
		if count.tests > maxTestFilesPerDir {
			t.Errorf("%s has %d test Go files, want <= %d", relPath(t, serverRoot, dir), count.tests, maxTestFilesPerDir)
		}
	}
}

func TestManagementPackagesDoNotLeakIntoDomainPackages(t *testing.T) {
	serverRoot := testServerRoot(t)
	internalRoot := filepath.Join(serverRoot, "internal")

	walkGoFiles(t, internalRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || pathWithin(path, filepath.Join(internalRoot, "app")) || pathWithin(path, filepath.Join(internalRoot, "management")) {
			return
		}

		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s imports: %v", relPath(t, serverRoot, path), err)
		}
		for _, imported := range parsed.Imports {
			importPath := strings.Trim(imported.Path.Value, `"`)
			if strings.HasPrefix(importPath, managementImportPrefix) {
				t.Errorf("%s imports management package %s", relPath(t, serverRoot, path), importPath)
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
