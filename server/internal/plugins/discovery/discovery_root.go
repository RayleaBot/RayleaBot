package discovery

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/logpath"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginmanifest "github.com/RayleaBot/RayleaBot/server/internal/plugins/manifest"
	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

func discoverRoot(root ScanRoot, validator *schema.Validator, repoRoot string, maxSummaryChars int, logger *slog.Logger) ([]plugins.Snapshot, int, error) {
	if logger != nil {
		logger.Info(
			fmt.Sprintf("开始扫描插件来源：%s（目录：%s）", root.Label, logpath.Display(repoRoot, root.Path)),
			"component", "plugins",
			"source_root", root.Label,
			"source_path", logpath.Display(repoRoot, root.Path),
		)
	}

	dirEntries, err := os.ReadDir(root.Path)
	if err != nil {
		if os.IsNotExist(err) {
			if logger != nil {
				logger.Info(
					fmt.Sprintf("插件来源目录不存在，已跳过：%s（目录：%s）", root.Label, logpath.Display(repoRoot, root.Path)),
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
						fmt.Sprintf("插件目录缺少 info.json，已跳过：%s", logpath.Display(repoRoot, pluginDir)),
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

		snapshot, ok, err := pluginmanifest.LoadSnapshot(infoPath, root.Label, repoRoot, validator, maxSummaryChars, logger)
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
