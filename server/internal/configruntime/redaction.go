package configruntime

import (
	"slices"
	"strings"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

const redactedConfigValue = "********"

func sanitizeConfigDocument(document map[string]any) (map[string]any, []string) {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil, nil
	}

	paths := configSecretPathsIn(cloned)
	redactedFields := make([]string, 0, len(paths))
	for _, path := range paths {
		value, ok := lookupConfigPath(cloned, path)
		if !ok || strings.TrimSpace(stringValue(value)) == "" {
			continue
		}
		setConfigPath(cloned, path, redactedConfigValue)
		redactedFields = append(redactedFields, strings.Join(path, "."))
	}
	slices.Sort(redactedFields)
	return cloned, redactedFields
}

func restoreRedactedConfigSecrets(request, current map[string]any) map[string]any {
	cloned := internalconfig.CloneDocument(request)
	if cloned == nil {
		return nil
	}

	// Resolve against the request: an adapter the caller did not send has no
	// secrets to restore into it.
	for _, path := range configSecretPathsIn(cloned) {
		requestValue, exists := lookupConfigPath(cloned, path)
		if exists && strings.TrimSpace(stringValue(requestValue)) != redactedConfigValue {
			continue
		}
		// A section the request omitted entirely is one the caller is not
		// configuring; restoring into it would materialise a half-built block
		// that then fails that section's own required fields. Within a section
		// the request did send, an omitted field still inherits its secret.
		if !exists && !configSectionPresent(cloned, path) {
			continue
		}
		currentValue, _ := lookupConfigPath(current, path)
		setConfigPath(cloned, path, stringValue(currentValue))
	}
	return cloned
}

// configSectionPresent reports whether the request holds the section the secret
// lives in. For a secret inside a collection entry that section is the entry's
// settings block; otherwise it is the top-level section.
func configSectionPresent(document map[string]any, path []string) bool {
	sectionEnd := 1
	for index := 1; index < len(path); index++ {
		if _, ok := ConfigCollectionKey(ConfigShapePath(strings.Join(path[:index], "."))); ok {
			// path[index] names an entry, so the section is the block inside it.
			sectionEnd = index + 2
		}
	}
	if sectionEnd >= len(path) {
		return document != nil
	}
	section, ok := lookupConfigPath(document, path[:sectionEnd])
	if !ok {
		return false
	}
	_, isSection := section.(map[string]any)
	return isSection
}

func configSecretValues(cfg internalconfig.Config) []string {
	document := ConfigDocumentFromTyped(cfg)
	paths := configSecretPathsIn(document)
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		value, ok := lookupConfigPath(document, path)
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

func lookupConfigPath(document map[string]any, path []string) (any, bool) {
	if len(path) == 0 {
		return document, true
	}

	current := any(document)
	for index, segment := range path {
		// A segment addressing a keyed collection names an entry by its own
		// identifier rather than by position, so reordering does not move it.
		if entries, ok := current.([]any); ok {
			entry, ok := collectionEntry(entries, collectionKeyFor(path[:index]), segment)
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

func collectionEntry(entries []any, key, wanted string) (map[string]any, bool) {
	for _, entry := range entries {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := item[key].(string); ok && id == wanted {
			return item, true
		}
	}
	return nil, false
}

func setConfigPath(document map[string]any, path []string, value any) {
	if document == nil || len(path) == 0 {
		return
	}

	current := any(document)
	for index, segment := range path[:len(path)-1] {
		if entries, ok := current.([]any); ok {
			entry, ok := collectionEntry(entries, collectionKeyFor(path[:index]), segment)
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
