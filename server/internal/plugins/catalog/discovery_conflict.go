package catalog

import (
	"log/slog"
	"sort"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func buildConflictSnapshot(pluginID string, group []plugins.Snapshot) plugins.Snapshot {
	conflictPaths := make([]string, 0, len(group))
	sourceRoots := make([]string, 0, len(group))
	for _, entry := range group {
		conflictPaths = append(conflictPaths, entry.ManifestPath)
		if !containsString(sourceRoots, entry.SourceRoot) {
			sourceRoots = append(sourceRoots, entry.SourceRoot)
		}
	}

	sort.Strings(conflictPaths)
	sort.Strings(sourceRoots)

	return plugins.Snapshot{
		PluginID:          pluginID,
		ManifestPath:      "",
		PackageRootPath:   "",
		SourceRoot:        "",
		SourceRoots:       sourceRoots,
		Valid:             false,
		ValidationSummary: "多个目录中发现相同插件 ID",
		RegistrationState: plugins.RegistrationStateInstalled,
		DesiredState:      plugins.DesiredStateDisabled,
		RuntimeState:      plugins.RuntimeStateStopped,
		DisplayState:      plugins.DisplayStateConflict,
		ConflictPaths:     conflictPaths,
	}
}

func shouldSkipPluginDiscoveryDir(name string) bool {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, ".plugin-install-") {
		return true
	}
	switch name {
	case "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache":
		return true
	default:
		return false
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func logPluginDiscovered(logger *slog.Logger, entry plugins.Snapshot) {
	if logger == nil {
		return
	}

	logger.Debug(
		"发现插件",
		"component", "plugins",
		"plugin_id", entry.PluginID,
		"plugin_name", entry.Name,
		"manifest_path", entry.ManifestPath,
		"source_root", entry.SourceRoot,
	)
}

func logPluginInvalid(logger *slog.Logger, entry plugins.Snapshot) {
	if logger == nil {
		return
	}

	logger.Warn(
		"插件配置无效，无法加载",
		"component", "plugins",
		"plugin_id", entry.PluginID,
		"plugin_name", entry.Name,
		"manifest_path", entry.ManifestPath,
		"source_root", entry.SourceRoot,
		"validation_summary", entry.ValidationSummary,
	)
}

func logPluginConflict(logger *slog.Logger, entry plugins.Snapshot) {
	if logger == nil {
		return
	}

	logger.Warn(
		"插件重复安装，无法加载；请仅保留一份",
		"component", "plugins",
		"plugin_id", entry.PluginID,
		"count", len(entry.ConflictPaths),
		"source_roots", strings.Join(entry.SourceRoots, ","),
	)
}
