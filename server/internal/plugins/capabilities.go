package plugins

import "strings"

func dedupeCapabilities(values []string) []string {
	items := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func DedupeCapabilities(values []string) []string {
	return dedupeCapabilities(values)
}

func CapabilityDeclared(snapshot Snapshot, capability string) bool {
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return false
	}
	for _, declared := range snapshot.DeclaredCapabilities {
		if strings.TrimSpace(declared) == capability {
			return true
		}
	}
	return false
}
