package templates

import (
	"path/filepath"
	"strings"
)

func ResolveAssetPath(root Root, relativePath string) (string, error) {
	templateDir := strings.TrimSpace(root.TemplateDir)
	resourceRoot := strings.TrimSpace(root.ResourceRoot)
	relativePath = strings.TrimSpace(relativePath)
	if templateDir == "" || resourceRoot == "" || relativePath == "" || filepath.IsAbs(filepath.FromSlash(relativePath)) {
		return "", &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}

	cleanRelative := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRelative == "." {
		return "", &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}

	absoluteTemplateDir, err := filepath.Abs(templateDir)
	if err != nil {
		return "", err
	}
	absoluteResourceRoot, err := filepath.Abs(resourceRoot)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(absoluteTemplateDir, cleanRelative)
	if !pathWithinRoot(absoluteResourceRoot, candidate) {
		return "", &Error{Code: "platform.resource_missing", Message: "render template asset was not found"}
	}
	return candidate, nil
}

func ManagedSourcePaths(templateDir string, files TemplateFiles) []string {
	relativePaths := []string{
		ManifestFilename,
		DefaultPreviewData,
	}
	if strings.TrimSpace(files.HTML) != "" {
		relativePaths = append(relativePaths, files.HTML)
	}
	if strings.TrimSpace(files.Stylesheet) != "" {
		relativePaths = append(relativePaths, files.Stylesheet)
	}
	if files.InputSchema != nil && strings.TrimSpace(*files.InputSchema) != "" {
		relativePaths = append(relativePaths, *files.InputSchema)
	}

	paths := make([]string, 0, len(relativePaths))
	seen := map[string]struct{}{}
	for _, relativePath := range relativePaths {
		path, err := TemplateFilePath(templateDir, relativePath)
		if err != nil {
			continue
		}
		absolutePath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		key := normalizedFilePath(absolutePath)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		paths = append(paths, absolutePath)
	}
	return paths
}

func SameFilePath(left, right string) bool {
	return normalizedFilePath(left) == normalizedFilePath(right)
}

func normalizedFilePath(path string) string {
	cleaned := filepath.Clean(path)
	if strings.Contains(cleaned, "\\") {
		return strings.ToLower(cleaned)
	}
	return cleaned
}

func pathWithinRoot(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
