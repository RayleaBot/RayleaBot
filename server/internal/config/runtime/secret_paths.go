package runtime

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
	index := configDocumentIndex{document: document}
	return index.secretPaths()
}

func (d *configDocumentIndex) secretPaths() [][]string {
	shapes := ConfigSecretFieldPaths()
	type resolvedPath struct {
		path []string
		name string
	}
	resolved := make([]resolvedPath, 0, len(shapes))
	for _, shape := range shapes {
		for _, path := range d.expandSecretShape(strings.Split(shape, ".")) {
			resolved = append(resolved, resolvedPath{path: path, name: strings.Join(path, ".")})
		}
	}
	slices.SortFunc(resolved, func(left, right resolvedPath) int {
		return strings.Compare(left.name, right.name)
	})
	paths := make([][]string, len(resolved))
	for index, path := range resolved {
		paths[index] = path.path
	}
	return paths
}

func (d *configDocumentIndex) expandSecretShape(shape []string) [][]string {
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
			for _, entryKey := range d.collectionEntryKeys(prefix, key) {
				branch := append(append([]string{}, prefix...), entryKey)
				// Entries of one collection hold different shapes: an adapter
				// speaking one protocol has no settings block for another. A
				// branch that cannot reach the field is not a path to it.
				if index+1 < len(shape) {
					probe := append(append([]string{}, branch...), shape[index+1])
					if _, ok := d.lookup(probe); !ok {
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
func (d *configDocumentIndex) collectionEntryKeys(path []string, key string) []string {
	value, ok := d.lookup(path)
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
