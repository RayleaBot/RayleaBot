package architecture_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type architectureBudget struct {
	Metrics                              map[string]metricBudget `json:"metrics"`
	PackageInternalFanOut                map[string]metricBudget `json:"package_internal_fan_out"`
	SingleFileProductionPackageAllowlist map[string]string       `json:"single_file_production_package_allowlist"`
}

type metricBudget struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

func TestCompositionRootSubtreeFanOutDoesNotExceedBudget(t *testing.T) {
	serverRoot := testServerRoot(t)
	budget := loadArchitectureBudget(t, serverRoot)
	appRoot := filepath.Join(serverRoot, "internal", "app")
	imports := map[string]struct{}{}

	walkGoFiles(t, appRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || isGeneratedGoFile(path) {
			return
		}
		for _, importPath := range fileImports(t, serverRoot, path) {
			if !strings.HasPrefix(importPath, modulePrefix) {
				continue
			}
			relImport := strings.TrimPrefix(importPath, "github.com/RayleaBot/RayleaBot/server/")
			if strings.HasPrefix(relImport, "internal/app") {
				continue
			}
			imports[relImport] = struct{}{}
		}
	})

	assertMetricWithinBudget(t, budget, "internal_app_external_internal_import_union", len(imports))
}

func TestCompositionRootSubpackageFanOutDoesNotExceedBudget(t *testing.T) {
	serverRoot := testServerRoot(t)
	budget := loadArchitectureBudget(t, serverRoot)
	fanOut := productionPackageInternalImports(t, serverRoot)

	for relDir, metric := range budget.PackageInternalFanOut {
		got := len(fanOut[filepath.ToSlash(relDir)])
		if got > metric.Max {
			t.Errorf("%s imports %d internal packages, budget is %d", relDir, got, metric.Max)
		}
	}
}

func TestInternalPackageCountsDoNotExceedBudget(t *testing.T) {
	serverRoot := testServerRoot(t)
	budget := loadArchitectureBudget(t, serverRoot)
	packages := productionPackageFileCounts(t, serverRoot)

	singleFilePackages := 0
	twoFilePackages := 0
	moduleGoSingleFilePackages := 0
	for relDir, count := range packages {
		if count == 1 {
			singleFilePackages++
			if productionPackageFileNames(t, serverRoot, relDir)[0] == "module.go" {
				moduleGoSingleFilePackages++
			}
		}
		if count == 2 {
			twoFilePackages++
		}
	}
	assertMetricWithinBudget(t, budget, "production_package_count", len(packages))
	assertMetricWithinBudget(t, budget, "single_file_production_package_count", singleFilePackages)
	assertMetricWithinBudget(t, budget, "two_file_production_package_count", twoFilePackages)
	assertMetricWithinBudget(t, budget, "module_go_single_file_package_count", moduleGoSingleFilePackages)
}

func TestSingleFileProductionPackagesAreAllowlisted(t *testing.T) {
	serverRoot := testServerRoot(t)
	budget := loadArchitectureBudget(t, serverRoot)
	packages := productionPackageFileCounts(t, serverRoot)
	for relDir, count := range packages {
		if count != 1 {
			continue
		}
		reason := strings.TrimSpace(budget.SingleFileProductionPackageAllowlist[relDir])
		if reason == "" {
			t.Errorf("single-file production package %s is not allowlisted with a reason", relDir)
		}
	}
	for relDir := range budget.SingleFileProductionPackageAllowlist {
		if packages[relDir] != 1 {
			t.Errorf("single-file package allowlist references %s, but current production file count is %d", relDir, packages[relDir])
		}
	}
}

func TestServerDirectoryCountDoesNotExceedBudget(t *testing.T) {
	serverRoot := testServerRoot(t)
	budget := loadArchitectureBudget(t, serverRoot)
	var count int
	if err := filepath.WalkDir(serverRoot, func(path string, entry os.DirEntry, err error) error {
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
		if path != serverRoot {
			count++
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", serverRoot, err)
	}
	assertMetricWithinBudget(t, budget, "server_directory_count", count)
}

func loadArchitectureBudget(t *testing.T, serverRoot string) architectureBudget {
	t.Helper()
	path := filepath.Join(serverRoot, "..", "docs", "engineering", "server-architecture-budget.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read architecture budget: %v", err)
	}
	var budget architectureBudget
	if err := json.Unmarshal(data, &budget); err != nil {
		t.Fatalf("decode architecture budget: %v", err)
	}
	return budget
}

func assertMetricWithinBudget(t *testing.T, budget architectureBudget, name string, got int) {
	t.Helper()
	metric, ok := budget.Metrics[name]
	if !ok {
		t.Fatalf("architecture budget missing metric %s", name)
	}
	if got > metric.Max {
		t.Errorf("%s = %d, budget is %d", name, got, metric.Max)
	}
}

func productionPackageFileCounts(t *testing.T, serverRoot string) map[string]int {
	t.Helper()
	internalRoot := filepath.Join(serverRoot, "internal")
	packages := map[string]int{}
	walkGoFiles(t, internalRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || isGeneratedGoFile(path) {
			return
		}
		relDir := relPath(t, serverRoot, filepath.Dir(path))
		packages[relDir]++
	})
	return packages
}

func productionPackageFileNames(t *testing.T, serverRoot, relDir string) []string {
	t.Helper()
	dir := filepath.Join(serverRoot, filepath.FromSlash(relDir))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read package dir %s: %v", relDir, err)
	}
	names := []string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if isGeneratedGoFile(path) {
			continue
		}
		names = append(names, entry.Name())
	}
	return names
}

func productionPackageInternalImports(t *testing.T, serverRoot string) map[string]map[string]struct{} {
	t.Helper()
	internalRoot := filepath.Join(serverRoot, "internal")
	fanOut := map[string]map[string]struct{}{}
	walkGoFiles(t, internalRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") || isGeneratedGoFile(path) {
			return
		}
		relDir := relPath(t, serverRoot, filepath.Dir(path))
		imports := fanOut[relDir]
		if imports == nil {
			imports = map[string]struct{}{}
			fanOut[relDir] = imports
		}
		for _, importPath := range fileImports(t, serverRoot, path) {
			if strings.HasPrefix(importPath, modulePrefix) {
				imports[strings.TrimPrefix(importPath, "github.com/RayleaBot/RayleaBot/server/")] = struct{}{}
			}
		}
	})
	return fanOut
}
