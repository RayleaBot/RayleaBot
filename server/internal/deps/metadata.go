package deps

import (
	"encoding/json"
	"github.com/RayleaBot/RayleaBot/server/internal/fsguard"
	"net/url"
	"strings"
)

func MetadataComplete(resource *Resource) bool {
	if resource == nil {
		return false
	}
	payload, err := json.Marshal(Manifest{ManifestVersion: ManifestVersion, Resources: []Resource{*resource}})
	return err == nil && validateManifestJSON(payload) == nil
}

func SourcesComplete(resource *Resource) bool {
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
		if err != nil || parsedURL.Scheme != "https" || parsedURL.Hostname() == "" || parsedURL.User != nil || parsedURL.Fragment != "" {
			return false
		}
		if !ValidSourceKind(strings.TrimSpace(source.Kind)) {
			return false
		}
		if _, ok := seen[rawURL]; ok {
			return false
		}
		seen[rawURL] = struct{}{}
	}
	return true
}

func ValidSourceKind(kind string) bool {
	switch kind {
	case "upstream", "mirror":
		return true
	default:
		return false
	}
}

func ArchiveFormatSupported(format string) bool {
	switch strings.TrimSpace(format) {
	case "zip", "tar.gz", "tar.xz":
		return true
	default:
		return false
	}
}

func HasRequiredEntrypoints(resource *Resource) bool {
	required := RequiredEntrypoints(resource)
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
		for _, candidate := range candidates {
			clean, err := fsguard.ArchivePath(candidate, true)
			if err != nil || clean != candidate {
				return false
			}
		}
	}
	return true
}

func RequiredEntrypoints(resource *Resource) []string {
	if resource == nil {
		return nil
	}
	switch resource.Kind {
	case "chromium":
		return []string{"browser"}
	case "ffmpeg":
		return []string{"ffmpeg", "ffprobe"}
	default:
		return nil
	}
}

func ResourceMetadataComplete(resource *Resource) bool {
	return MetadataComplete(resource)
}

func requiredEntrypoints(resource *Resource) []string {
	return RequiredEntrypoints(resource)
}
