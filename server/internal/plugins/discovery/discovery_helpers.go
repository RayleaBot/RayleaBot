package discovery

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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}

	return false
}

func pathWithinRoot(root, candidate string) bool {
	relativePath, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relativePath == "." || (relativePath != "" && relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator)))
}
