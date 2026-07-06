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
		options.Logger.Info(
			fmt.Sprintf(
				"插件扫描完成：有效 %d 个，无效 %d 个，冲突 %d 个，跳过 %d 个",
				summary.ValidCount,
				summary.InvalidCount,
				summary.ConflictCount,
				summary.SkippedCount,
			),
			"component", "plugins",
			"valid_count", summary.ValidCount,
			"invalid_count", summary.InvalidCount,
			"conflict_count", summary.ConflictCount,
			"skipped_count", summary.SkippedCount,
		)
	}

	return snapshots, summary, nil
}
