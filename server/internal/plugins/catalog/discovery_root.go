package catalog

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func discoverRoot(root ScanRoot, validator *config.Validator, repoRoot string, maxSummaryChars int, logger *slog.Logger) ([]plugins.Snapshot, int, error) {
	if logger != nil {
		logger.Debug(
			"开始扫描插件来源",
			"component", "plugins",
			"source_root", root.Label,
			"source_path", logpath.Display(repoRoot, root.Path),
		)
	}

	dirEntries, err := os.ReadDir(root.Path)
	if err != nil {
		if os.IsNotExist(err) {
			if logger != nil {
				logger.Debug(
					"插件来源目录不存在，已跳过",
					"component", "plugins",
					"source_root", root.Label,
					"source_path", logpath.Display(repoRoot, root.Path),
				)
			}
			return nil, 0, nil
		}

		return nil, 0, fmt.Errorf("read plugin root %s: %w", root.Path, err)
	}

	sort.Slice(dirEntries, func(i, j int) bool {
		return dirEntries[i].Name() < dirEntries[j].Name()
	})

	var snapshots []plugins.Snapshot
	skipped := 0

	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		if shouldSkipPluginDiscoveryDir(dirEntry.Name()) {
			continue
		}

		pluginDir := filepath.Join(root.Path, dirEntry.Name())
		infoPath := filepath.Join(pluginDir, "info.json")
		if _, err := os.Stat(infoPath); err != nil {
			if os.IsNotExist(err) {
				skipped++
				if logger != nil {
					logger.Warn(
						"插件目录缺少 info.json，无法加载",
						"component", "plugins",
						"plugin_dir", logpath.Display(repoRoot, pluginDir),
						"manifest_path", logpath.Display(repoRoot, infoPath),
						"source_root", root.Label,
					)
				}
				continue
			}

			return nil, skipped, fmt.Errorf("stat %s: %w", infoPath, err)
		}

		snapshot, ok, err := LoadSnapshot(infoPath, root.Label, repoRoot, validator, maxSummaryChars, logger)
		if err != nil {
			return nil, skipped, err
		}
		if !ok {
			skipped++
			continue
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, skipped, nil
}
