package spec

import (
	"path/filepath"
	"strings"
)

func resolveManifestPath(repoRoot, manifestPath string) string {
	if filepath.IsAbs(manifestPath) {
		return manifestPath
	}
	if repoRoot == "" {
		return filepath.Clean(filepath.FromSlash(manifestPath))
	}
	return filepath.Join(repoRoot, filepath.FromSlash(manifestPath))
}

func escapesDir(root, path string) bool {
	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return true
	}
	return relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}

func resolveSymlinkTarget(entryPath, linkTarget string) string {
	if filepath.IsAbs(linkTarget) {
		return filepath.Clean(linkTarget)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(entryPath), linkTarget))
}
