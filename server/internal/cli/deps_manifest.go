package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type depsManifest struct {
	ManifestVersion int                    `json:"manifest_version"`
	Resources       []depsManifestResource `json:"resources"`
}

type depsManifestResource struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Source   string `json:"source"`
	SHA256   string `json:"sha256"`
}

func loadDepsManifest(repoRoot string) (*depsManifest, error) {
	manifestPath := filepath.Join(repoRoot, ".deps", "manifest.json")
	payload, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest depsManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return nil, fmt.Errorf("decode deps manifest: %w", err)
	}
	return &manifest, nil
}

func currentManifestPlatform() string {
	return manifestPlatform(runtime.GOOS, runtime.GOARCH)
}

func manifestPlatform(goos, goarch string) string {
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

func (manifest *depsManifest) hasPlatform(platform string) bool {
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

func (manifest *depsManifest) findResource(platform, kind string) *depsManifestResource {
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

func manifestResourceMetadataComplete(resource *depsManifestResource) bool {
	if resource == nil {
		return false
	}
	source := strings.TrimSpace(resource.Source)
	if source == "" || strings.Contains(strings.ToUpper(source), "TODO(") {
		return false
	}
	parsedURL, err := url.Parse(source)
	if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
		return false
	}
	sha256 := strings.ToLower(strings.TrimSpace(resource.SHA256))
	if strings.Contains(strings.ToUpper(sha256), "TODO(") {
		return false
	}
	return sha256Pattern.MatchString(sha256)
}
