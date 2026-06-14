package deps

import depsmanifest "github.com/RayleaBot/RayleaBot/server/internal/deps/manifest"

func ResourceMetadataComplete(resource *Resource) bool {
	return depsmanifest.MetadataComplete(resource)
}

func resourceSourcesComplete(resource *Resource) bool {
	return depsmanifest.SourcesComplete(resource)
}

func validResourceSourceKind(kind string) bool {
	return depsmanifest.ValidSourceKind(kind)
}

func archiveFormatSupported(format string) bool {
	return depsmanifest.ArchiveFormatSupported(format)
}

func resourceHasRequiredEntrypoints(resource *Resource) bool {
	return depsmanifest.HasRequiredEntrypoints(resource)
}

func requiredEntrypoints(resource *Resource) []string {
	return depsmanifest.RequiredEntrypoints(resource)
}
