package pluginmanifest

import (
	"path/filepath"
	"strings"
)

func trimSummary(summary string, maxLen int) string {
	singleLine := strings.Join(strings.Fields(summary), " ")
	if len(singleLine) <= maxLen {
		return singleLine
	}

	return singleLine[:maxLen-3] + "..."
}

func displayPath(repoRoot, path string) string {
	if repoRoot != "" {
		relativePath, err := filepath.Rel(repoRoot, path)
		if err == nil && relativePath != "." && !strings.HasPrefix(relativePath, "..") {
			return filepath.ToSlash(relativePath)
		}
	}

	return filepath.ToSlash(path)
}

func pathWithinRoot(root, candidate string) bool {
	relativePath, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relativePath == "." || (relativePath != "" && relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator)))
}
