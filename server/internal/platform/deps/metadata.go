package deps

import (
	"encoding/json"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/fsguard"
)

func MetadataComplete(resource *Resource) bool {
	if resource == nil {
		return false
	}
	payload, err := json.Marshal(Manifest{ManifestVersion: ManifestVersion, Resources: []Resource{*resource}})
	return err == nil && validateManifestJSON(payload) == nil
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
