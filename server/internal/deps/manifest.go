package deps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const ManifestVersion = 3

func LoadManifest(repoRoot string) (*Manifest, error) {
	return LoadManifestPath(filepath.Join(strings.TrimSpace(repoRoot), ".deps", "manifest.json"))
}
func LoadManifestPath(manifestPath string) (*Manifest, error) {
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return nil, fmt.Errorf("decode deps manifest: %w", err)
	}
	if manifest.ManifestVersion != ManifestVersion {
		return nil, fmt.Errorf("unsupported deps manifest version %d", manifest.ManifestVersion)
	}
	return &manifest, nil
}
func CurrentPlatform() string {
	return ManifestPlatform(runtime.GOOS, runtime.GOARCH)
}
func ManifestPlatform(goos, goarch string) string {
	switch goos {
	case "windows":
		return "windows-" + normalizeManifestArch(goarch)
	case "darwin":
		return "macos-" + normalizeManifestArch(goarch)
	default:
		return goos + "-" + normalizeManifestArch(goarch)
	}
}
func normalizeManifestArch(goarch string) string {
	switch goarch {
	case "amd64":
		return "x64"
	default:
		return goarch
	}
}
func (manifest *Manifest) HasPlatform(platform string) bool {
	if manifest == nil {
		return false
	}
	for _, resource := range manifest.Resources {
		if resource.Platform == platform {
			return true
		}
	}
	return false
}
func (manifest *Manifest) FindResource(platform, kind string) *Resource {
	if manifest == nil {
		return nil
	}
	for i := range manifest.Resources {
		resource := &manifest.Resources[i]
		if resource.Platform == platform && resource.Kind == kind {
			return resource
		}
	}
	return nil
}
