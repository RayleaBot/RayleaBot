package plugins

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

func discoverRoot(root ScanRoot, validator *schema.Validator, repoRoot string, maxSummaryChars int, logger *slog.Logger) ([]Snapshot, int, error) {
	if logger != nil {
		logger.Info(
			"plugin discovery starting",
			"component", "plugins",
			"source_root", root.Label,
		)
	}

	dirEntries, err := os.ReadDir(root.Path)
	if err != nil {
		if os.IsNotExist(err) {
			if logger != nil {
				logger.Info(
					"plugin source root missing, skipping",
					"component", "plugins",
					"source_root", root.Label,
				)
			}
			return nil, 0, nil
		}

		return nil, 0, fmt.Errorf("read plugin root %s: %w", root.Path, err)
	}

	sort.Slice(dirEntries, func(i, j int) bool {
		return dirEntries[i].Name() < dirEntries[j].Name()
	})

	var snapshots []Snapshot
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
						"plugin directory skipped because info.json is missing",
						"component", "plugins",
						"plugin_dir", displayPath(repoRoot, pluginDir),
						"manifest_path", displayPath(repoRoot, infoPath),
						"source_root", root.Label,
					)
				}
				continue
			}

			return nil, skipped, fmt.Errorf("stat %s: %w", infoPath, err)
		}

		snapshot, ok, err := loadSnapshot(infoPath, root.Label, repoRoot, validator, maxSummaryChars, logger)
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
