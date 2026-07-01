package deps

import depsmanifest "github.com/RayleaBot/RayleaBot/server/internal/deps/manifest"

func ResourceMetadataComplete(resource *Resource) bool {
	return depsmanifest.MetadataComplete(resource)
}

func requiredEntrypoints(resource *Resource) []string {
	return depsmanifest.RequiredEntrypoints(resource)
}
