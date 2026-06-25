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

	maxInternalProductionPackages   = 153
	maxSingleFileProductionPackages = 48
	maxGenericHelperFiles           = 0
	maxGenericServiceOrTypesFiles   = 0
	maxTestFilesOver600Lines        = 22
	maxTestFilesOver1000Lines       = 0
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
// the app composition root:
// platform -> pluginstack -> renderstack -> eventstack -> servicegraph -> httpwire.
// A lower layer must never import a higher one, and only internal/app itself may
// reach across all composition sub-packages.
func TestCompositionRootLayering(t *testing.T) {
	serverRoot := testServerRoot(t)
	appRoot := filepath.Join(serverRoot, "internal", "app")

	const (
		platform     = appImportPrefix + "/platform"
		pluginstack  = appImportPrefix + "/pluginstack"
		renderstack  = appImportPrefix + "/renderstack"
		eventstack   = appImportPrefix + "/eventstack"
		servicegraph = appImportPrefix + "/servicegraph"
		httpwire     = appImportPrefix + "/httpwire"
	)
	// forbidden maps a sub-package directory to the higher layers it must not import.
	forbidden := map[string][]string{
		"platform":     {pluginstack, renderstack, eventstack, servicegraph, httpwire},
		"pluginstack":  {renderstack, eventstack, servicegraph, httpwire},
		"renderstack":  {eventstack, servicegraph, httpwire},
		"eventstack":   {servicegraph, httpwire},
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

func TestCompositionRootFanOutDoesNotExceedBudget(t *testing.T) {
	serverRoot := testServerRoot(t)
	budget := loadArchitectureBudget(t, serverRoot)
	budgetKeys := []string{
		"internal/app/servicegraph",
		"internal/app/httpwire",
	}

	for _, budgetKey := range budgetKeys {
		metric, ok := budget.PackageInternalFanOut[budgetKey]
		if !ok {
			t.Fatalf("architecture budget missing package_internal_fan_out.%s", budgetKey)
		}
		dir := filepath.Join(serverRoot, filepath.FromSlash(budgetKey))
		imports := map[string]struct{}{}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", budgetKey, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			for _, importPath := range fileImports(t, serverRoot, path) {
				if strings.HasPrefix(importPath, modulePrefix) {
					imports[strings.TrimPrefix(importPath, "github.com/RayleaBot/RayleaBot/server/")] = struct{}{}
				}
			}
		}
		if len(imports) > metric.Max {
			t.Errorf("%s imports %d internal packages, budget is %d", budgetKey, len(imports), metric.Max)
		}
	}
}

func TestPluginStackDoesNotImportEventRenderOrGovernanceWiring(t *testing.T) {
	serverRoot := testServerRoot(t)
	pluginStackRoot := filepath.Join(serverRoot, "internal", "app", "pluginstack")
	forbidden := []string{
		modulePrefix + "bot/adapter/",
		modulePrefix + "eventpipeline/bridge",
		modulePrefix + "eventpipeline/dispatch",
		modulePrefix + "eventpipeline/eventingress",
		modulePrefix + "eventpipeline/outbound",
		modulePrefix + "permission",
		modulePrefix + "render/",
	}

	walkGoFiles(t, pluginStackRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		for _, importPath := range fileImports(t, serverRoot, path) {
			for _, forbiddenImport := range forbidden {
				if importPath == forbiddenImport || strings.HasPrefix(importPath, forbiddenImport) {
					t.Errorf("%s imports non-plugin wiring package %s", relPath(t, serverRoot, path), importPath)
				}
			}
		}
	})
}

func TestEventPipelinePackagesAreGrouped(t *testing.T) {
	serverRoot := testServerRoot(t)
	requiredDirs := []string{"eventingress", "chatpolicy", "bridge", "dispatch", "outbound"}
	for _, leaf := range requiredDirs {
		relDir := filepath.Join("internal", "eventpipeline", leaf)
		if !hasProductionGoFile(t, filepath.Join(serverRoot, relDir)) {
			t.Errorf("event pipeline package %s has no production Go file", filepath.ToSlash(relDir))
		}
		if _, err := os.Stat(filepath.Join(serverRoot, "internal", leaf)); err == nil {
			t.Errorf("legacy top-level event package internal/%s still exists", leaf)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect legacy event package internal/%s: %v", leaf, err)
		}
	}
}

func TestPluginSubsystemResponsibilityPackagesExist(t *testing.T) {
	serverRoot := testServerRoot(t)
	requiredDirs := map[string]string{
		"manifest":       filepath.Join("internal", "plugins", "manifest"),
		"catalog":        filepath.Join("internal", "plugins", "catalog"),
		"install":        filepath.Join("internal", "plugins", "install"),
		"runtime":        filepath.Join("internal", "plugins", "runtime"),
		"actions":        filepath.Join("internal", "plugins", "actions"),
		"config storage": filepath.Join("internal", "plugins", "configstore"),
		"file storage":   filepath.Join("internal", "plugins", "filestore"),
		"kv storage":     filepath.Join("internal", "plugins", "kvstore"),
		"webhook":        filepath.Join("internal", "plugins", "webhook"),
		"management ui":  filepath.Join("internal", "plugins", "managementui"),
	}

	for label, relDir := range requiredDirs {
		dir := filepath.Join(serverRoot, relDir)
		if !hasProductionGoFile(t, dir) {
			t.Errorf("plugin %s package %s has no production Go file", label, filepath.ToSlash(relDir))
		}
	}
}

func TestRenderImplementationPackagesStayBehindServiceBoundary(t *testing.T) {
	serverRoot := testServerRoot(t)
	internalRoot := filepath.Join(serverRoot, "internal")
	renderRoot := filepath.Join(internalRoot, "render")
	protectedPrefixes := []string{
		modulePrefix + "render/catalog",
		modulePrefix + "render/engine",
		modulePrefix + "render/pluginsync",
		modulePrefix + "render/plugintemplates",
		modulePrefix + "render/repository",
		modulePrefix + "render/templates",
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

func TestGenericProductionFilenamesDoNotExceedBudget(t *testing.T) {
	serverRoot := testServerRoot(t)
	internalRoot := filepath.Join(serverRoot, "internal")
	helperFiles := 0
	serviceOrTypesFiles := 0

	walkGoFiles(t, internalRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || isGeneratedGoFile(path) {
			return
		}
		switch filepath.Base(path) {
		case "helper.go", "helpers.go":
			helperFiles++
		case "service.go", "types.go":
			serviceOrTypesFiles++
		}
	})

	if helperFiles > maxGenericHelperFiles {
		t.Errorf("generic helper filenames = %d, budget is %d", helperFiles, maxGenericHelperFiles)
	}
	if serviceOrTypesFiles > maxGenericServiceOrTypesFiles {
		t.Errorf("generic service/types filenames = %d, budget is %d", serviceOrTypesFiles, maxGenericServiceOrTypesFiles)
	}
}

func TestOversizedTestFilesDoNotExceedBudget(t *testing.T) {
	serverRoot := testServerRoot(t)
	var over600 int
	var over1000 int

	walkGoFiles(t, serverRoot, func(path string) {
		if !strings.HasSuffix(path, "_test.go") {
			return
		}
		lineCount, err := countLines(path)
		if err != nil {
			t.Fatalf("count %s lines: %v", relPath(t, serverRoot, path), err)
		}
		if lineCount > 600 {
			over600++
		}
		if lineCount > 1000 {
			over1000++
		}
	})

	if over600 > maxTestFilesOver600Lines {
		t.Errorf("test files over 600 lines = %d, budget is %d", over600, maxTestFilesOver600Lines)
	}
	if over1000 > maxTestFilesOver1000Lines {
		t.Errorf("test files over 1000 lines = %d, budget is %d", over1000, maxTestFilesOver1000Lines)
	}
}

func TestTestFilesUseScenarioNames(t *testing.T) {
	serverRoot := testServerRoot(t)

	walkGoFiles(t, serverRoot, func(path string) {
		if !strings.HasSuffix(path, "_test.go") {
			return
		}
		name := filepath.Base(path)
		if strings.Contains(name, "_part2_") || strings.Contains(name, "_part3_") || strings.Contains(name, "_part4_") {
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

func hasProductionGoFile(t *testing.T, root string) bool {
	t.Helper()

	found := false
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") && !isGeneratedGoFile(path) {
			found = true
			return filepath.SkipAll
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return found
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
	if strings.Contains(normalized, "/internal/sqlcgen/") || strings.Contains(normalized, "/internal/schemaassets/") {
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
