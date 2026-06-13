package pluginui

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func pluginUIAssetRoot(snapshot plugins.Snapshot) string {
	if snapshot.ManagementUI == nil || strings.TrimSpace(snapshot.PackageRootPath) == "" {
		return ""
	}

	if len(snapshot.ManagementUI.Pages) == 0 {
		return ""
	}
	entryDir := path.Dir(strings.TrimSpace(snapshot.ManagementUI.Pages[0].Entry))
	if entryDir == "." || entryDir == "/" {
		return filepath.Clean(snapshot.PackageRootPath)
	}
	return filepath.Clean(filepath.Join(snapshot.PackageRootPath, filepath.FromSlash(entryDir)))
}

func normalizePluginUIAssetPath(assetPath string) string {
	cleaned := path.Clean("/" + strings.TrimSpace(assetPath))
	if cleaned == "/" || cleaned == "." {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func isPathWithinRoot(root, candidate string) bool {
	relativePath, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relativePath == "." || (!strings.HasPrefix(relativePath, "..") && relativePath != "")
}
