package deps

import depsmanifest "github.com/RayleaBot/RayleaBot/server/internal/deps/manifest"

const ManifestVersion = depsmanifest.ManifestVersion

func LoadManifest(repoRoot string) (*Manifest, error) {
	return depsmanifest.Load(repoRoot)
}

func LoadManifestPath(manifestPath string) (*Manifest, error) {
	return depsmanifest.LoadPath(manifestPath)
}

func CurrentPlatform() string {
	return depsmanifest.CurrentPlatform()
}

func ManifestPlatform(goos, goarch string) string {
	return depsmanifest.Platform(goos, goarch)
}

func normalizeManifestArch(goarch string) string {
	return depsmanifest.NormalizeArch(goarch)
}
