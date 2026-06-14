package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const ManifestVersion = 3

type Manifest struct {
	ManifestVersion int        `json:"manifest_version"`
	Resources       []Resource `json:"resources"`
}

type ResourceSource struct {
	URL   string `json:"url"`
	Kind  string `json:"kind"`
	Label string `json:"label,omitempty"`
}

type Resource struct {
	ID            string              `json:"id"`
	Kind          string              `json:"kind"`
	Version       string              `json:"version"`
	Platform      string              `json:"platform"`
	Sources       []ResourceSource    `json:"sources"`
	SHA256        string              `json:"sha256"`
	ArchiveFormat string              `json:"archive_format"`
	Entrypoints   map[string][]string `json:"entrypoints"`
}

func Load(repoRoot string) (*Manifest, error) {
	return LoadPath(filepath.Join(strings.TrimSpace(repoRoot), ".deps", "manifest.json"))
}

func LoadPath(manifestPath string) (*Manifest, error) {
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
	return Platform(runtime.GOOS, runtime.GOARCH)
}

func Platform(goos, goarch string) string {
	switch goos {
	case "windows":
		return "windows-" + NormalizeArch(goarch)
	case "darwin":
		return "macos-" + NormalizeArch(goarch)
	default:
		return goos + "-" + NormalizeArch(goarch)
	}
}

func NormalizeArch(goarch string) string {
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
