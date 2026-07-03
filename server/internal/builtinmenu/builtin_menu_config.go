package menu

import (
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func builtinMenuPrefixes(cfg config.Config) []string {
	if len(cfg.Builtin.Menu.Prefixes) > 0 {
		return sanitizeCommandPrefixes(cfg.Builtin.Menu.Prefixes)
	}
	return runtimeCommandPrefixes(cfg)
}

func runtimeCommandPrefixes(cfg config.Config) []string {
	if cfg.Command != nil && len(cfg.Command.Prefixes) > 0 {
		return sanitizeCommandPrefixes(cfg.Command.Prefixes)
	}
	return []string{"/"}
}

func sanitizeCommandPrefixes(prefixes []string) []string {
	items := make([]string, 0, len(prefixes))
	seen := make(map[string]struct{}, len(prefixes))
	for _, prefix := range prefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if _, ok := seen[prefix]; ok {
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

func builtinMenuCommands(cfg config.Config) []string {
	items := sanitizeMenuTokens(cfg.Builtin.Menu.Commands)
	if len(items) == 0 {
		return []string{"help", "帮助"}
	}
	return items
}

func sanitizeMenuTokens(values []string) []string {
	items := make([]string, 0, len(values))
	seen := map[string]struct{}{}
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
