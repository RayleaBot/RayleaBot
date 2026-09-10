package config

import "strings"

// CommandPrefixes returns the effective host command prefixes for every consumer.
func (c Config) CommandPrefixes() []string {
	if c.Command != nil {
		return NormalizeCommandPrefixes(c.Command.Prefixes)
	}
	return NormalizeCommandPrefixes(nil)
}

// NormalizeCommandPrefixes removes empty and duplicate entries and applies the default.
func NormalizeCommandPrefixes(prefixes []string) []string {
	items := make([]string, 0, len(prefixes))
	seen := make(map[string]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if _, exists := seen[prefix]; exists {
			continue
		}
		seen[prefix] = struct{}{}
		items = append(items, prefix)
	}
	if len(items) == 0 {
		return []string{"/"}
	}
	return items
}
