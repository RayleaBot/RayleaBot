package configruntime

import (
	"maps"
	"reflect"
	"slices"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

type ConfigApplyPolicy string

const (
	ConfigApplyPolicyHotReload       ConfigApplyPolicy = "hot_reload"
	ConfigApplyPolicyAdapterReload   ConfigApplyPolicy = "adapter_reload"
	ConfigApplyPolicyRestartRequired ConfigApplyPolicy = "restart_required"
	ConfigApplyPolicySecretOnly      ConfigApplyPolicy = "secret_only"
	ConfigApplyPolicyReadOnly        ConfigApplyPolicy = "read_only"
)

func ClassifyApplyEffects(oldCfg internalconfig.Config, newCfg internalconfig.Config) ApplyEffects {
	effects := NewApplyEffects()

	for _, path := range diffConfigDocumentPaths(ConfigDocumentFromTyped(oldCfg), ConfigDocumentFromTyped(newCfg)) {
		policy, ok := ConfigApplyPolicyForPath(path)
		switch {
		case !ok:
			effects.RestartRequiredFields = append(effects.RestartRequiredFields, path)
		case policy == ConfigApplyPolicyAdapterReload || policy == ConfigApplyPolicySecretOnly:
			effects.ReloadedNow = append(effects.ReloadedNow, path)
		case policy == ConfigApplyPolicyRestartRequired || policy == ConfigApplyPolicyReadOnly:
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
		keys := make(map[string]struct{}, configDiffKeyCapacity(len(currentMap), len(nextMap)))
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

func configDiffKeyCapacity(currentCount int, nextCount int) int {
	maxInt := int(^uint(0) >> 1)
	if currentCount < 0 || nextCount < 0 || currentCount > maxInt-nextCount {
		return currentCount
	}
	return currentCount + nextCount
}

func joinConfigPath(prefix string, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func normalizeConfigEffectPaths(paths []string) []string {
	if len(paths) == 0 {
		return []string{}
	}

	normalized := append([]string(nil), paths...)
	slices.Sort(normalized)
	return slices.Compact(normalized)
}
