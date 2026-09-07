package configruntime

import (
	"slices"
	"strings"
)

// Secret field paths come in two forms. A shape path may contain
// ConfigCollectionWildcard where a keyed collection sits; a concrete path names
// one actual field in one document, with each wildcard replaced by the entry's
// key. Only concrete paths address a value.

// configSecretPathsIn resolves the secret shapes against one document. A shape
// crossing a collection expands to one path per entry, keyed by the entry's own
// identifier rather than its position, so reordering the collection does not
// re-key its secrets.
func configSecretPathsIn(document map[string]any) [][]string {
	resolved := make([][]string, 0)
	for _, shape := range ConfigSecretFieldPaths() {
		resolved = append(resolved, expandSecretShape(document, strings.Split(shape, "."))...)
	}
	slices.SortFunc(resolved, func(left, right []string) int {
		return strings.Compare(strings.Join(left, "."), strings.Join(right, "."))
	})
	return resolved
}

func expandSecretShape(document map[string]any, shape []string) [][]string {
	prefixes := [][]string{{}}
	for index, segment := range shape {
		if segment != ConfigCollectionWildcard {
			for i := range prefixes {
				prefixes[i] = append(prefixes[i], segment)
			}
			continue
		}
		// The wildcard stands for the entries of the collection named by the
		// path so far; each contributes one branch keyed by its identifier.
		collectionPath := strings.Join(shape[:index], ".")
		key, ok := ConfigCollectionKey(collectionPath)
		if !ok {
			return nil
		}
		expanded := make([][]string, 0, len(prefixes))
		for _, prefix := range prefixes {
			for _, entryKey := range collectionEntryKeys(document, prefix, key) {
				branch := append(append([]string{}, prefix...), entryKey)
				// Entries of one collection hold different shapes: an adapter
				// speaking one protocol has no settings block for another. A
				// branch that cannot reach the field is not a path to it.
				if index+1 < len(shape) {
					probe := append(append([]string{}, branch...), shape[index+1])
					if _, ok := lookupConfigPath(document, probe); !ok {
						continue
					}
				}
				expanded = append(expanded, branch)
			}
		}
		prefixes = expanded
		if len(prefixes) == 0 {
			return nil
		}
	}
	return prefixes
}

// collectionEntryKeys reads the identifiers of the entries actually present at
// a collection path.
func collectionEntryKeys(document map[string]any, path []string, key string) []string {
	value, ok := lookupConfigPath(document, path)
	if !ok {
		return nil
	}
	entries, ok := value.([]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := item[key].(string); ok && strings.TrimSpace(id) != "" {
			keys = append(keys, strings.TrimSpace(id))
		}
	}
	return keys
}
