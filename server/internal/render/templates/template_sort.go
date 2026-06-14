package templates

import "sort"

func SortedIDs(seeds map[string]Seed) []string {
	ids := make([]string, 0, len(seeds))
	for templateID := range seeds {
		ids = append(ids, templateID)
	}
	sort.Strings(ids)
	return ids
}
