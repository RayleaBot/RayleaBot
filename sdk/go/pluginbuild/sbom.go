package pluginbuild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type moduleInfo struct {
	Path    string
	Version string
	Main    bool
}

type dependencyInfo struct {
	Name    string
	Version string
}

func goModules(ctx context.Context, config Config, pluginDir string) ([]moduleInfo, error) {
	command := config.GoCommand
	if command == "" {
		command = "go"
	}
	cmd := exec.CommandContext(ctx, command, "list", "-m", "-json", "all")
	cmd.Dir = pluginDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pluginbuild: list Go modules: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var modules []moduleInfo
	for {
		var module moduleInfo
		if err := decoder.Decode(&module); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("pluginbuild: decode Go module list: %w", err)
		}
		if module.Main {
			continue
		}
		if module.Version == "" {
			module.Version = "NOASSERTION"
		}
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })
	return modules, nil
}

func readUIDependencies(pluginDir string) ([]dependencyInfo, error) {
	path := filepath.Join(pluginDir, "ui", "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var document struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("pluginbuild: parse UI package.json: %w", err)
	}
	combined := make(map[string]string, len(document.Dependencies)+len(document.DevDependencies))
	for name, version := range document.Dependencies {
		combined[name] = version
	}
	for name, version := range document.DevDependencies {
		combined[name] = version
	}
	items := make([]dependencyInfo, 0, len(combined))
	for name, version := range combined {
		items = append(items, dependencyInfo{Name: name, Version: version})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func writeSBOM(ctx context.Context, config Config, pluginDir, root string, manifest Manifest) error {
	modules, err := goModules(ctx, config, pluginDir)
	if err != nil {
		return err
	}
	uiDependencies, err := readUIDependencies(pluginDir)
	if err != nil {
		return err
	}
	packages := []map[string]any{{
		"SPDXID":           "SPDXRef-Package-" + manifest.ID,
		"name":             manifest.ID,
		"versionInfo":      manifest.Version,
		"downloadLocation": "NOASSERTION",
		"filesAnalyzed":    false,
		"licenseConcluded": "NOASSERTION",
		"licenseDeclared":  "NOASSERTION",
	}}
	for index, module := range modules {
		packages = append(packages, map[string]any{
			"SPDXID":           fmt.Sprintf("SPDXRef-GoModule-%d", index+1),
			"name":             module.Path,
			"versionInfo":      module.Version,
			"downloadLocation": "NOASSERTION",
			"filesAnalyzed":    false,
			"licenseConcluded": "NOASSERTION",
			"licenseDeclared":  "NOASSERTION",
		})
	}
	for index, dependency := range uiDependencies {
		packages = append(packages, map[string]any{
			"SPDXID":           fmt.Sprintf("SPDXRef-NPMPackage-%d", index+1),
			"name":             dependency.Name,
			"versionInfo":      dependency.Version,
			"downloadLocation": "NOASSERTION",
			"filesAnalyzed":    false,
			"licenseConcluded": "NOASSERTION",
			"licenseDeclared":  "NOASSERTION",
		})
	}
	document := map[string]any{
		"spdxVersion":       "SPDX-2.3",
		"dataLicense":       "CC0-1.0",
		"SPDXID":            "SPDXRef-DOCUMENT",
		"name":              manifest.ID + "-" + manifest.Version,
		"documentNamespace": "https://rayleabot.local/sbom/" + manifest.ID + "/" + manifest.Version,
		"creationInfo": map[string]any{
			"created":  time.Unix(0, 0).UTC().Format(time.RFC3339),
			"creators": []string{"Tool: RayleaBot pluginbuild"},
		},
		"packages": packages,
	}
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(root, "sbom.spdx.json"), data, 0o644)
}
