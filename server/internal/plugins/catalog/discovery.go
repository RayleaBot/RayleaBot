package catalog

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type ScanRoot struct {
	Label string
	Path  string
}

type DiscoverOptions struct {
	Validator       *config.Validator
	Roots           []ScanRoot
	RepoRoot        string
	Logger          *slog.Logger
	MaxSummaryChars int
}

type DiscoverSummary struct {
	ValidCount    int
	InvalidCount  int
	ConflictCount int
	SkippedCount  int
}

func Discover(options DiscoverOptions) ([]plugins.Snapshot, DiscoverSummary, error) {
	if options.Validator == nil {
		return nil, DiscoverSummary{}, fmt.Errorf("plugin manifest validator is required")
	}

	maxSummaryChars := options.MaxSummaryChars
	if maxSummaryChars <= 0 {
		maxSummaryChars = plugins.ManifestValidationMaxSummary
	}

	var summary DiscoverSummary
	byPluginID := map[string][]plugins.Snapshot{}

	for _, root := range options.Roots {
		entries, skipped, err := discoverRoot(root, options.Validator, options.RepoRoot, maxSummaryChars, options.Logger)
		if err != nil {
			return nil, summary, err
		}

		summary.SkippedCount += skipped
		for _, entry := range entries {
			byPluginID[entry.PluginID] = append(byPluginID[entry.PluginID], entry)
		}
	}

	pluginIDs := make([]string, 0, len(byPluginID))
	for pluginID := range byPluginID {
		pluginIDs = append(pluginIDs, pluginID)
	}
	sort.Strings(pluginIDs)

	snapshots := make([]plugins.Snapshot, 0, len(pluginIDs))
	for _, pluginID := range pluginIDs {
		group := byPluginID[pluginID]
		if len(group) == 1 {
			entry := group[0]
			if entry.Valid {
				summary.ValidCount++
				logPluginDiscovered(options.Logger, entry)
			} else {
				summary.InvalidCount++
				logPluginInvalid(options.Logger, entry)
			}
			snapshots = append(snapshots, entry)
			continue
		}

		conflictSnapshot := buildConflictSnapshot(pluginID, group)
		summary.ConflictCount++
		logPluginConflict(options.Logger, conflictSnapshot)
		snapshots = append(snapshots, conflictSnapshot)
	}

	if options.Logger != nil {
		message := fmt.Sprintf("发现 %d 个可用插件", summary.ValidCount)
		if summary.InvalidCount > 0 {
			message += fmt.Sprintf("，%d 个配置无效", summary.InvalidCount)
		}
		if summary.ConflictCount > 0 {
			message += fmt.Sprintf("，%d 个重复安装", summary.ConflictCount)
		}
		if summary.SkippedCount > 0 {
			message += fmt.Sprintf("，%d 个目录已跳过", summary.SkippedCount)
		}
		options.Logger.Info(
			message,
			"component", "plugins",
			"valid_count", summary.ValidCount,
			"invalid_count", summary.InvalidCount,
			"conflict_count", summary.ConflictCount,
			"skipped_count", summary.SkippedCount,
		)
	}

	return snapshots, summary, nil
}
