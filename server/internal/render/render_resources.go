package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var supportedRenderResourceMIMEs = map[string]string{
	"image/gif":  ".gif",
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

func normalizeRenderResources(resources []RenderResource) ([]RenderResource, error) {
	if len(resources) == 0 {
		return nil, nil
	}
	result := make([]RenderResource, 0, len(resources))
	seen := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		resource.ID = strings.TrimSpace(resource.ID)
		resource.Path = filepath.Clean(strings.TrimSpace(resource.Path))
		resource.MIME = strings.ToLower(strings.TrimSpace(resource.MIME))
		resource.SHA256 = strings.ToLower(strings.TrimSpace(resource.SHA256))
		if resource.ID == "" || !filepath.IsAbs(resource.Path) {
			return nil, fmt.Errorf("render resource %q is invalid", resource.ID)
		}
		if _, exists := seen[resource.ID]; exists {
			return nil, fmt.Errorf("render resource %q is duplicated", resource.ID)
		}
		seen[resource.ID] = struct{}{}
		if _, ok := supportedRenderResourceMIMEs[resource.MIME]; !ok {
			return nil, fmt.Errorf("render resource %q has unsupported MIME %q", resource.ID, resource.MIME)
		}
		if len(resource.SHA256) != sha256.Size*2 {
			return nil, fmt.Errorf("render resource %q has invalid digest", resource.ID)
		}
		if _, err := hex.DecodeString(resource.SHA256); err != nil {
			return nil, fmt.Errorf("render resource %q has invalid digest: %w", resource.ID, err)
		}
		info, err := os.Stat(resource.Path)
		if err != nil {
			return nil, fmt.Errorf("stat render resource %q: %w", resource.ID, err)
		}
		if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() != resource.Size {
			return nil, fmt.Errorf("render resource %q has invalid size", resource.ID)
		}
		result = append(result, resource)
	}
	return result, nil
}

func renderResourcesDigest(resources []RenderResource) string {
	if len(resources) == 0 {
		return "none"
	}
	items := append([]RenderResource(nil), resources...)
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	hash := sha256.New()
	for _, resource := range items {
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00%s\x00%d\n", resource.ID, resource.MIME, resource.SHA256, resource.Size)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func renderResourceExtension(mime string) (string, bool) {
	extension, ok := supportedRenderResourceMIMEs[strings.ToLower(strings.TrimSpace(mime))]
	return extension, ok
}
