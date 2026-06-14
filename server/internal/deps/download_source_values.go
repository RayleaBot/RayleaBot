package deps

import depsdownload "github.com/RayleaBot/RayleaBot/server/internal/deps/download"

func normalizedResourceSources(sources []ResourceSource) []ResourceSource {
	return depsdownload.NormalizeSources(sources)
}

func downloadSourceSummary(kind string, source ResourceSource) string {
	return depsdownload.SourceSummary(kind, source)
}
