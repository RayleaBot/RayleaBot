package catalog

import (
	"fmt"
	"path/filepath"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/runtimepaths"
)

type DiscoverySpec struct {
	RepoRoot         string
	PluginSchemaPath string
	Roots            []ScanRoot
}

type DiscoveryOptions struct {
	ConfigPath       string
	PluginRepoRoot   string
	PluginSchemaPath string
	PluginRoots      []ScanRoot
}

func ResolveDiscovery(options DiscoveryOptions) (DiscoverySpec, error) {
	if len(options.PluginRoots) > 0 || options.PluginRepoRoot != "" || options.PluginSchemaPath != "" {
		if options.PluginRepoRoot == "" || options.PluginSchemaPath == "" || len(options.PluginRoots) == 0 {
			return DiscoverySpec{}, fmt.Errorf("plugin discovery override requires repo root, schema path, and roots")
		}
		return DiscoverySpec{
			RepoRoot:         options.PluginRepoRoot,
			PluginSchemaPath: options.PluginSchemaPath,
			Roots:            append([]ScanRoot(nil), options.PluginRoots...),
		}, nil
	}

	repoRoot, pluginSchemaPath, roots, err := DiscoveryContext(options.ConfigPath)
	if err != nil {
		return DiscoverySpec{}, err
	}
	return DiscoverySpec{
		RepoRoot:         repoRoot,
		PluginSchemaPath: pluginSchemaPath,
		Roots:            roots,
	}, nil
}

func DiscoveryContext(configPath string) (string, string, []ScanRoot, error) {
	repoRoot, err := runtimepaths.ResolveRuntimeRoot(configPath)
	if err != nil {
		return "", "", nil, err
	}
	pluginSchemaPath := config.PluginInfoSchemaID

	roots := []ScanRoot{
		{
			Label: "plugins/installed",
			Path:  filepath.Join(repoRoot, "plugins", "installed"),
		},
	}

	return repoRoot, pluginSchemaPath, roots, nil
}
