package cli

import "rayleabot/server/internal/deps"

type depsManifest deps.Manifest
type depsManifestResource deps.Resource

func loadDepsManifest(repoRoot string) (*depsManifest, error) {
	manifest, err := deps.LoadManifest(repoRoot)
	if err != nil {
		return nil, err
	}
	wrapped := depsManifest(*manifest)
	return &wrapped, nil
}

func currentManifestPlatform() string {
	return deps.CurrentPlatform()
}

func manifestPlatform(goos, goarch string) string {
	return deps.ManifestPlatform(goos, goarch)
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
		resource := (*depsManifestResource)(&manifest.Resources[i])
		if resource.Platform == platform && resource.Kind == kind {
			return resource
		}
	}
	return nil
}

func manifestResourceMetadataComplete(resource *depsManifestResource) bool {
	return deps.ResourceMetadataComplete((*deps.Resource)(resource))
}
