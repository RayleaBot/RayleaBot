package configruntime

import (
	"maps"
	"reflect"
	"slices"
	"strings"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

func ClassifyApplyEffects(oldCfg internalconfig.Config, newCfg internalconfig.Config) ApplyEffects {
	effects := NewApplyEffects()

	for _, path := range diffConfigDocumentPaths(ConfigDocumentFromTyped(oldCfg), ConfigDocumentFromTyped(newCfg)) {
		switch {
		case isConfigReloadPath(path):
			effects.ReloadedNow = append(effects.ReloadedNow, path)
		case isConfigRestartPath(path):
			effects.RestartRequiredFields = append(effects.RestartRequiredFields, path)
		default:
			effects.AppliedNow = append(effects.AppliedNow, path)
		}
	}

	normalizeConfigApplyEffects(&effects)
	return effects
}

func diffConfigDocumentPaths(current, next map[string]any) []string {
	paths := make([]string, 0)
	collectConfigPathChanges("", current, next, &paths)
	return normalizeConfigEffectPaths(paths)
}

func collectConfigPathChanges(prefix string, current, next any, paths *[]string) {
	currentMap, currentIsMap := current.(map[string]any)
	nextMap, nextIsMap := next.(map[string]any)
	if currentIsMap && nextIsMap {
		keys := make(map[string]struct{}, len(currentMap)+len(nextMap))
		for key := range currentMap {
			keys[key] = struct{}{}
		}
		for key := range nextMap {
			keys[key] = struct{}{}
		}
		sortedKeys := slices.Collect(maps.Keys(keys))
		slices.Sort(sortedKeys)
		for _, key := range sortedKeys {
			collectConfigPathChanges(joinConfigPath(prefix, key), currentMap[key], nextMap[key], paths)
		}
		return
	}

	if reflect.DeepEqual(current, next) || prefix == "" {
		return
	}

	*paths = append(*paths, prefix)
}

func joinConfigPath(prefix string, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func isConfigReloadPath(path string) bool {
	return strings.HasPrefix(path, "onebot.") || strings.HasPrefix(path, "adapter.")
}

func isConfigRestartPath(path string) bool {
	switch {
	case strings.HasPrefix(path, "server."):
		return true
	case strings.HasPrefix(path, "database."):
		return true
	case strings.HasPrefix(path, "web."):
		return true
	}

	switch path {
	case "admin.session_ttl_days",
		"admin.sliding_renewal",
		"admin.max_sessions",
		"render.worker_count",
		"render.browser_path",
		"render.browser_args":
		return true
	default:
		return false
	}
}

func normalizeConfigEffectPaths(paths []string) []string {
	if len(paths) == 0 {
		return []string{}
	}

	normalized := append([]string(nil), paths...)
	slices.Sort(normalized)
	return slices.Compact(normalized)
}
