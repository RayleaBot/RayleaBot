package recovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func BuildBackupManifest(repoRoot string, consistency string) BackupManifest {
	return BackupManifest{
		Version:             BackupManifestVersion,
		CreatedAt:           time.Now().UTC().Format(time.RFC3339),
		CoreVersion:         DetectCoreVersion(repoRoot),
		ConfigSchemaVersion: config.CurrentSchemaVersion(),
		DBSchemaVersion:     storage.CurrentSchemaVersion(),
		Consistency:         strings.TrimSpace(consistency),
		Plugins:             loadManifestPlugins(filepath.Join(repoRoot, "plugins", "installed")),
	}
}

func DetectCoreVersion(repoRoot string) string {
	buildInfoPath := filepath.Join(repoRoot, "build_info.json")
	payload, err := os.ReadFile(buildInfoPath)
	if err != nil {
		return defaultCoreVersion
	}
	var buildInfo struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(payload, &buildInfo); err != nil {
		return defaultCoreVersion
	}
	if strings.TrimSpace(buildInfo.Version) == "" {
		return defaultCoreVersion
	}
	return strings.TrimSpace(buildInfo.Version)
}

func Directory(path, label string) BackupManifestDirectory {
	return BackupManifestDirectory{Label: label, Path: filepath.ToSlash(path)}
}

func loadManifestPlugins(pluginsRoot string) []BackupManifestPlugin {
	entries, err := os.ReadDir(pluginsRoot)
	if err != nil {
		return nil
	}
	items := make([]BackupManifestPlugin, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		infoPath := filepath.Join(pluginsRoot, entry.Name(), "info.json")
		payload, err := os.ReadFile(infoPath)
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			continue
		}
		item := BackupManifestPlugin{
			PluginID:          stringValue(raw["id"]),
			Version:           stringValue(raw["version"]),
			MinCoreVersion:    stringValue(raw["min_core_version"]),
			DataSchemaVersion: stringValue(raw["data_schema_version"]),
			Platforms:         stringSlice(raw["platforms"]),
			SourceRoot:        "plugins/installed",
		}
		if item.PluginID == "" {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].PluginID < items[j].PluginID
	})
	return items
}

func ScanRepoPaths(repoRoot, configPath, databasePath string) []BackupManifestDirectory {
	items := make([]BackupManifestDirectory, 0, 3)
	if configPath != "" {
		if relative, err := filepath.Rel(repoRoot, configPath); err == nil {
			items = append(items, Directory(relative, "config"))
		}
	}
	if databasePath != "" {
		if relative, err := filepath.Rel(repoRoot, databasePath); err == nil {
			items = append(items, Directory(relative, "database"))
		}
	}
	pluginsPath := filepath.Join(repoRoot, "plugins", "installed")
	if info, err := os.Stat(pluginsPath); err == nil && info.IsDir() {
		if relative, err := filepath.Rel(repoRoot, pluginsPath); err == nil {
			items = append(items, Directory(relative, "plugins"))
		}
	}
	return items
}

func RepoRootFromConfigPath(configPath string) string {
	return filepath.Dir(filepath.Dir(configPath))
}
