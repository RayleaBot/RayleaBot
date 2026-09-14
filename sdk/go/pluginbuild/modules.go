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
	if os.Getenv("RAYLEA_PLUGIN_BUILD_USE_WORKSPACE") == "1" {
		return workspaceGoModules(ctx, command, pluginDir, config.BackendPackage)
	}
	cmd := exec.CommandContext(ctx, command, "list", "-m", "-json", "all")
	cmd.Dir = pluginDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pluginbuild: list Go modules: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return decodeGoModules(output, "")
}

func workspaceGoModules(ctx context.Context, command, pluginDir, backendPackage string) ([]moduleInfo, error) {
	modCommand := exec.CommandContext(ctx, command, "mod", "edit", "-json")
	modCommand.Dir = pluginDir
	modCommand.Env = append(os.Environ(), "GOWORK=off")
	modOutput, err := modCommand.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pluginbuild: read plugin go.mod: %w: %s", err, strings.TrimSpace(string(modOutput)))
	}
	var modDocument struct {
		Module struct {
			Path string `json:"Path"`
		} `json:"Module"`
		Require []struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
		} `json:"Require"`
	}
	if err := json.Unmarshal(modOutput, &modDocument); err != nil {
		return nil, fmt.Errorf("pluginbuild: decode plugin go.mod: %w", err)
	}
	requiredVersions := make(map[string]string, len(modDocument.Require))
	for _, requirement := range modDocument.Require {
		requiredVersions[requirement.Path] = requirement.Version
	}

	listCommand := exec.CommandContext(ctx, command, "list", "-deps", "-json", backendPackage)
	listCommand.Dir = pluginDir
	output, err := listCommand.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pluginbuild: list workspace Go dependencies: %w: %s", err, strings.TrimSpace(string(output)))
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	modulesByPath := map[string]moduleInfo{}
	for {
		var item struct {
			Module *moduleInfo `json:"Module"`
		}
		if err := decoder.Decode(&item); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("pluginbuild: decode workspace Go dependency list: %w", err)
		}
		if item.Module == nil || item.Module.Path == "" || item.Module.Path == modDocument.Module.Path {
			continue
		}
		module := *item.Module
		if module.Version == "" {
			module.Version = requiredVersions[module.Path]
		}
		if module.Version == "" {
			module.Version = "NOASSERTION"
		}
		modulesByPath[module.Path] = module
	}
	modules := make([]moduleInfo, 0, len(modulesByPath))
	for _, module := range modulesByPath {
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })
	return modules, nil
}

func decodeGoModules(output []byte, mainPath string) ([]moduleInfo, error) {
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var modules []moduleInfo
	for {
		var module moduleInfo
		if err := decoder.Decode(&module); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("pluginbuild: decode Go module list: %w", err)
		}
		if module.Main || module.Path == mainPath {
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
