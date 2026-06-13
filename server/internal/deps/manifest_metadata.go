package deps

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func ResourceMetadataComplete(resource *Resource) bool {
	if resource == nil {
		return false
	}
	if strings.TrimSpace(resource.ArchiveFormat) == "" {
		return false
	}
	if !archiveFormatSupported(resource.ArchiveFormat) {
		return false
	}
	if !resourceHasRequiredEntrypoints(resource) {
		return false
	}
	if !resourceSourcesComplete(resource) {
		return false
	}
	sha256 := strings.ToLower(strings.TrimSpace(resource.SHA256))
	if strings.Contains(strings.ToUpper(sha256), "TODO(") {
		return false
	}
	return sha256Pattern.MatchString(sha256)
}

func resourceSourcesComplete(resource *Resource) bool {
	if resource == nil || len(resource.Sources) == 0 {
		return false
	}
	seen := map[string]struct{}{}
	for _, source := range resource.Sources {
		rawURL := strings.TrimSpace(source.URL)
		if rawURL == "" || strings.Contains(strings.ToUpper(rawURL), "TODO(") {
			return false
		}
		parsedURL, err := url.Parse(rawURL)
		if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
			return false
		}
		if !validResourceSourceKind(strings.TrimSpace(source.Kind)) {
			return false
		}
		if _, ok := seen[rawURL]; ok {
			return false
		}
		seen[rawURL] = struct{}{}
	}
	return true
}

func validResourceSourceKind(kind string) bool {
	switch kind {
	case "upstream", "mirror":
		return true
	default:
		return false
	}
}

func archiveFormatSupported(format string) bool {
	switch strings.TrimSpace(format) {
	case "zip", "tar.gz", "tar.xz":
		return true
	default:
		return false
	}
}

func resourceHasRequiredEntrypoints(resource *Resource) bool {
	required := requiredEntrypoints(resource)
	if len(required) == 0 {
		return false
	}
	if len(resource.Entrypoints) == 0 {
		return false
	}
	for _, key := range required {
		candidates := resource.Entrypoints[key]
		if len(candidates) == 0 {
			return false
		}
		valid := false
		for _, candidate := range candidates {
			clean := strings.TrimSpace(candidate)
			if clean == "" {
				continue
			}
			if filepath.IsAbs(clean) {
				continue
			}
			if clean == "." || strings.HasPrefix(clean, "..") {
				continue
			}
			valid = true
			break
		}
		if !valid {
			return false
		}
	}
	return true
}

func requiredEntrypoints(resource *Resource) []string {
	if resource == nil {
		return nil
	}
	switch resource.Kind {
	case "chromium":
		return []string{"browser"}
	case "python-runtime":
		return []string{"python"}
	case "nodejs-runtime":
		return []string{"node", "npm"}
	default:
		return nil
	}
}
