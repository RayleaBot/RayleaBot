package runtime

import (
	"slices"
	"strings"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

const redactedConfigValue = "********"

func sanitizeConfigDocument(document map[string]any) (map[string]any, []string) {
	return sanitizeOwnedConfigDocument(internalconfig.CloneDocument(document))
}

func sanitizeOwnedConfigDocument(document map[string]any) (map[string]any, []string) {
	if document == nil {
		return nil, nil
	}

	index := configDocumentIndex{document: document}
	paths := index.secretPaths()
	redactedFields := make([]string, 0, len(paths))
	for _, path := range paths {
		value, ok := index.lookup(path)
		if !ok || strings.TrimSpace(stringValue(value)) == "" {
			continue
		}
		index.set(path, redactedConfigValue)
		redactedFields = append(redactedFields, strings.Join(path, "."))
	}
	slices.Sort(redactedFields)
	return document, redactedFields
}

func restoreRedactedConfigSecrets(request, current map[string]any) map[string]any {
	cloned := internalconfig.CloneDocument(request)
	if cloned == nil {
		return nil
	}
	requestIndex := configDocumentIndex{document: cloned}
	currentIndex := configDocumentIndex{document: current}

	// Resolve against the request: an adapter the caller did not send has no
	// secrets to restore into it.
	for _, path := range requestIndex.secretPaths() {
		requestValue, exists := requestIndex.lookup(path)
		if exists && strings.TrimSpace(stringValue(requestValue)) != redactedConfigValue {
			continue
		}
		// A section the request omitted entirely is one the caller is not
		// configuring; restoring into it would materialise a half-built block
		// that then fails that section's own required fields. Within a section
		// the request did send, an omitted field still inherits its secret.
		if !exists && !requestIndex.sectionPresent(path) {
			continue
		}
		currentValue, _ := currentIndex.lookup(path)
		requestIndex.set(path, stringValue(currentValue))
	}
	return cloned
}

// sectionPresent reports whether the request holds the section the secret
// lives in. For a secret inside a collection entry that section is the entry's
// settings block; otherwise it is the top-level section.
func (d *configDocumentIndex) sectionPresent(path []string) bool {
	sectionEnd := 1
	for index := 1; index < len(path); index++ {
		if _, ok := ConfigCollectionKey(ConfigShapePath(strings.Join(path[:index], "."))); ok {
			// path[index] names an entry, so the section is the block inside it.
			sectionEnd = index + 2
		}
	}
	if sectionEnd >= len(path) {
		return d.document != nil
	}
	section, ok := d.lookup(path[:sectionEnd])
	if !ok {
		return false
	}
	_, isSection := section.(map[string]any)
	return isSection
}

func configSecretValues(cfg internalconfig.Config) []string {
	document := ConfigDocumentFromTyped(cfg)
	index := configDocumentIndex{document: document}
	paths := index.secretPaths()
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		value, ok := index.lookup(path)
		if !ok {
			continue
		}
		// An unset secret carries no value to hide, and registering the empty
		// string would make the redactor match everywhere.
		secret := stringValue(value)
		if secret == "" {
			continue
		}
		values = append(values, secret)
	}
	return values
}

// The index lives for one secret operation. That operation changes only secret
// leaves, never collection membership or the identifiers used by this cache.
type configDocumentIndex struct {
	document    map[string]any
	collections map[string]map[string]map[string]any
}

func lookupConfigPath(document map[string]any, path []string) (any, bool) {
	index := configDocumentIndex{document: document}
	return index.lookup(path)
}

func (d *configDocumentIndex) lookup(path []string) (any, bool) {
	if len(path) == 0 {
		return d.document, true
	}

	current := any(d.document)
	for index, segment := range path {
		// A segment addressing a keyed collection names an entry by its own
		// identifier rather than by position, so reordering does not move it.
		if entries, ok := current.([]any); ok {
			entry, ok := d.collectionEntry(entries, path[:index], segment)
			if !ok {
				return nil, false
			}
			current = entry
			continue
		}
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := currentMap[segment]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

// collectionKeyFor reports which field names the entries of the collection at
// this path, defaulting to "id" when the schema declares none.
func collectionKeyFor(path []string) string {
	if key, ok := ConfigCollectionKey(strings.Join(path, ".")); ok {
		return key
	}
	return "id"
}

func (d *configDocumentIndex) collectionEntry(entries []any, path []string, wanted string) (map[string]any, bool) {
	collectionPath := strings.Join(path, ".")
	indexed, ok := d.collections[collectionPath]
	if !ok {
		key := collectionKeyFor(path)
		indexed = make(map[string]map[string]any, len(entries))
		for _, entry := range entries {
			item, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			id, ok := item[key].(string)
			if _, exists := indexed[id]; ok && !exists {
				indexed[id] = item
			}
		}
		if d.collections == nil {
			d.collections = make(map[string]map[string]map[string]any)
		}
		d.collections[collectionPath] = indexed
	}
	entry, ok := indexed[wanted]
	return entry, ok
}

func setConfigPath(document map[string]any, path []string, value any) {
	index := configDocumentIndex{document: document}
	index.set(path, value)
}

func (d *configDocumentIndex) set(path []string, value any) {
	if d.document == nil || len(path) == 0 {
		return
	}

	current := any(d.document)
	for index, segment := range path[:len(path)-1] {
		if entries, ok := current.([]any); ok {
			entry, ok := d.collectionEntry(entries, path[:index], segment)
			if !ok {
				// Never invent a collection entry: it would have no identity.
				return
			}
			current = entry
			continue
		}
		currentMap, ok := current.(map[string]any)
		if !ok {
			return
		}
		next, ok := currentMap[segment].(map[string]any)
		if !ok {
			if _, isList := currentMap[segment].([]any); isList {
				current = currentMap[segment]
				continue
			}
			next = map[string]any{}
			currentMap[segment] = next
		}
		current = next
	}
	if currentMap, ok := current.(map[string]any); ok {
		currentMap[path[len(path)-1]] = value
	}
}

func stringValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}
