package render

import "sort"

func sortedTemplateIDs(seeds map[string]templateSeed) []string {
	ids := make([]string, 0, len(seeds))
	for templateID := range seeds {
		ids = append(ids, templateID)
	}
	sort.Strings(ids)
	return ids
}
