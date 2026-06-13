package deps

import (
	"path/filepath"
	"strings"
)

func StoreRoot(repoRoot string, resource *Resource) string {
	if resource == nil {
		return ""
	}
	return filepath.Join(strings.TrimSpace(repoRoot), ".deps", "store", resource.ID, resource.Version)
}
func CacheRoot(repoRoot string) string {
	return filepath.Join(strings.TrimSpace(repoRoot), "cache", "downloads", "runtime")
}
func LockPath(repoRoot string) string {
	return filepath.Join(strings.TrimSpace(repoRoot), "cache", "downloads", "platform.lock")
}
func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
func archiveSuffix(format string) string {
	switch format {
	case "tar.gz":
		return ".tar.gz"
	case "tar.xz":
		return ".tar.xz"
	default:
		return ".zip"
	}
}
