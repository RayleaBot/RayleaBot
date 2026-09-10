package logpath

import (
	"path/filepath"
	"sort"
	"strings"
)

func Display(repoRoot, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if looksLikeURI(value) {
		return value
	}

	cleanValue := filepath.Clean(value)
	if !filepath.IsAbs(cleanValue) {
		return filepath.ToSlash(cleanValue)
	}

	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot != "" {
		cleanRoot := filepath.Clean(repoRoot)
		if !filepath.IsAbs(cleanRoot) {
			if absoluteRoot, err := filepath.Abs(cleanRoot); err == nil {
				cleanRoot = absoluteRoot
			}
		}
		if relativePath, err := filepath.Rel(cleanRoot, cleanValue); err == nil && isLocalRelative(relativePath) {
			return filepath.ToSlash(relativePath)
		}
	}

	return filepath.ToSlash(cleanValue)
}

func Error(repoRoot string, err error, paths ...string) string {
	if err == nil {
		return ""
	}
	return Text(repoRoot, err.Error(), paths...)
}

func Text(repoRoot, message string, paths ...string) string {
	paths = uniqueNonEmpty(paths...)
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) > len(paths[j])
	})
	for _, path := range paths {
		displayPath := Display(repoRoot, path)
		for _, variant := range pathVariants(path) {
			if variant == "" || variant == displayPath {
				continue
			}
			message = strings.ReplaceAll(message, variant, displayPath)
		}
	}
	return message
}

func isLocalRelative(value string) bool {
	value = filepath.Clean(value)
	return value == "." || (value != ".." && !strings.HasPrefix(value, ".."+string(filepath.Separator)))
}

func looksLikeURI(value string) bool {
	schemeEnd := strings.Index(value, "://")
	if schemeEnd <= 0 {
		return false
	}
	for _, r := range value[:schemeEnd] {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '+' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func pathVariants(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	cleanPath := filepath.Clean(path)
	slashPath := filepath.ToSlash(cleanPath)
	return uniqueNonEmpty(path, cleanPath, filepath.ToSlash(path), slashPath)
}

func uniqueNonEmpty(values ...string) []string {
	seen := make(map[string]bool, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
}
