package plugins

import (
	"log/slog"
	"sort"
	"strings"
)

func buildConflictSnapshot(pluginID string, group []Snapshot) Snapshot {
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

	return Snapshot{
		PluginID:          pluginID,
		ManifestPath:      "",
		PackageRootPath:   "",
		SourceRoot:        "",
		SourceRoots:       sourceRoots,
		Valid:             false,
		ValidationSummary: "duplicate plugin_id discovered across multiple directories",
		RegistrationState: stateInstalled,
		DesiredState:      stateDisabled,
		RuntimeState:      stateStopped,
		DisplayState:      displayConflict,
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

func logPluginDiscovered(logger *slog.Logger, entry Snapshot) {
	if logger == nil {
		return
	}

	logger.Info(
		"plugin discovered",
		"component", "plugins",
		"plugin_id", entry.PluginID,
		"manifest_path", entry.ManifestPath,
		"source_root", entry.SourceRoot,
	)
}

func logPluginInvalid(logger *slog.Logger, entry Snapshot) {
	if logger == nil {
		return
	}

	logger.Warn(
		"plugin manifest invalid",
		"component", "plugins",
		"plugin_id", entry.PluginID,
		"manifest_path", entry.ManifestPath,
		"source_root", entry.SourceRoot,
		"validation_summary", entry.ValidationSummary,
	)
}

func logPluginConflict(logger *slog.Logger, entry Snapshot) {
	if logger == nil {
		return
	}

	logger.Warn(
		"plugin id conflict",
		"component", "plugins",
		"plugin_id", entry.PluginID,
		"count", len(entry.ConflictPaths),
		"source_roots", strings.Join(entry.SourceRoots, ","),
	)
}
