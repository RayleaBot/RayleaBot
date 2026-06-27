package discovery

import (
	"fmt"
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
	switch strings.TrimSpace(name) {
	case "__pycache__", ".pytest_cache", ".mypy_cache", ".ruff_cache":
		return true
	default:
		return false
	}
}

func logPluginDiscovered(logger *slog.Logger, entry plugins.Snapshot) {
	if logger == nil {
		return
	}

	logger.Info(
		fmt.Sprintf("插件%s已发现：来源 %s，清单 %s", plugins.DisplayLabel(entry), entry.SourceRoot, entry.ManifestPath),
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
		fmt.Sprintf("插件%s清单校验失败：%s", plugins.DisplayLabel(entry), entry.ValidationSummary),
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
		fmt.Sprintf("插件 ID 冲突：%s 同时出现在 %d 个目录", entry.PluginID, len(entry.ConflictPaths)),
		"component", "plugins",
		"plugin_id", entry.PluginID,
		"count", len(entry.ConflictPaths),
		"source_roots", strings.Join(entry.SourceRoots, ","),
	)
}
