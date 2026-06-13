package deps

import "strings"

func normalizedResourceSources(sources []ResourceSource) []ResourceSource {
	normalized := make([]ResourceSource, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.URL) == "" {
			continue
		}
		source.URL = strings.TrimSpace(source.URL)
		source.Label = strings.TrimSpace(source.Label)
		source.Kind = strings.TrimSpace(source.Kind)
		normalized = append(normalized, source)
	}
	return normalized
}

func downloadSourceSummary(kind string, source ResourceSource) string {
	label := strings.TrimSpace(source.Label)
	if label == "" {
		return "正在下载 " + managedResourceLabel(kind)
	}
	return "正在从 " + label + " 下载 " + managedResourceLabel(kind)
}
