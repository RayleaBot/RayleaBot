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

var configApplyPolicies = map[string]ConfigApplyPolicy{
	"schema_version": ConfigApplyPolicyReadOnly,

	"server.host": ConfigApplyPolicyRestartRequired,
	"server.port": ConfigApplyPolicyRestartRequired,

	"onebot.reverse_ws.enabled":                   ConfigApplyPolicyAdapterReload,
	"onebot.reverse_ws.url":                       ConfigApplyPolicyAdapterReload,
	"onebot.reverse_ws.access_token":              ConfigApplyPolicySecretOnly,
	"onebot.reverse_ws.access_token_query_compat": ConfigApplyPolicyAdapterReload,
	"onebot.forward_ws.enabled":                   ConfigApplyPolicyAdapterReload,
	"onebot.forward_ws.url":                       ConfigApplyPolicyAdapterReload,
	"onebot.forward_ws.access_token":              ConfigApplyPolicySecretOnly,
	"onebot.forward_ws.access_token_query_compat": ConfigApplyPolicyAdapterReload,
	"onebot.http_api.enabled":                     ConfigApplyPolicyAdapterReload,
	"onebot.http_api.url":                         ConfigApplyPolicyAdapterReload,
	"onebot.http_api.access_token":                ConfigApplyPolicySecretOnly,
	"onebot.webhook.enabled":                      ConfigApplyPolicyAdapterReload,
	"onebot.webhook.url":                          ConfigApplyPolicyAdapterReload,
	"onebot.webhook.access_token":                 ConfigApplyPolicySecretOnly,
	"onebot.webhook.access_token_query_compat":    ConfigApplyPolicyAdapterReload,

	"database.engine": ConfigApplyPolicyRestartRequired,
	"database.path":   ConfigApplyPolicyRestartRequired,

	"command.prefixes":               ConfigApplyPolicyHotReload,
	"builtin_features.menu.commands": ConfigApplyPolicyHotReload,
	"builtin_features.menu.prefixes": ConfigApplyPolicyHotReload,

	"admin.super_admins":                ConfigApplyPolicyHotReload,
	"admin.session_ttl_days":            ConfigApplyPolicyRestartRequired,
	"admin.sliding_renewal":             ConfigApplyPolicyRestartRequired,
	"admin.max_sessions":                ConfigApplyPolicyRestartRequired,
	"admin.login_fail_limit":            ConfigApplyPolicyHotReload,
	"admin.login_fail_window_seconds":   ConfigApplyPolicyHotReload,
	"permission.default_level":          ConfigApplyPolicyHotReload,
	"render.worker_count":               ConfigApplyPolicyRestartRequired,
	"render.browser_args":               ConfigApplyPolicyRestartRequired,
	"render.browser_path":               ConfigApplyPolicyRestartRequired,
	"render.default_output":             ConfigApplyPolicyHotReload,
	"render.device_scale_percent":       ConfigApplyPolicyHotReload,
	"render.timeout_seconds":            ConfigApplyPolicyHotReload,
	"render.queue_wait_timeout_seconds": ConfigApplyPolicyHotReload,
	"render.queue_max_length":           ConfigApplyPolicyHotReload,
	"render.footer_template":            ConfigApplyPolicyHotReload,

	"scheduler.timezone": ConfigApplyPolicyRestartRequired,

	"runtime.plugin_init_timeout_seconds":           ConfigApplyPolicyRestartRequired,
	"runtime.plugin_init_max_total_seconds":         ConfigApplyPolicyRestartRequired,
	"runtime.plugin_event_timeout_seconds":          ConfigApplyPolicyRestartRequired,
	"runtime.max_pending_events_per_plugin":         ConfigApplyPolicyRestartRequired,
	"runtime.max_pending_control_events_per_plugin": ConfigApplyPolicyRestartRequired,
	"runtime.nodejs_max_old_space_size_mb":          ConfigApplyPolicyRestartRequired,
	"runtime.dependency_install_timeout_seconds":    ConfigApplyPolicyRestartRequired,
	"runtime.max_concurrent_dependency_installs":    ConfigApplyPolicyRestartRequired,
	"runtime.ipc_pending_actions_max":               ConfigApplyPolicyRestartRequired,
	"runtime.ipc_action_burst_limit":                ConfigApplyPolicyRestartRequired,
	"runtime.stderr_rate_limit_bytes_per_second":    ConfigApplyPolicyRestartRequired,
	"runtime.max_concurrent_tasks_per_plugin":       ConfigApplyPolicyRestartRequired,
	"runtime.crash_backoff_initial_seconds":         ConfigApplyPolicyRestartRequired,
	"runtime.crash_backoff_max_seconds":             ConfigApplyPolicyRestartRequired,
	"runtime.shutdown_grace_seconds":                ConfigApplyPolicyRestartRequired,
	"runtime.ipc_message_max_bytes":                 ConfigApplyPolicyRestartRequired,

	"storage.kv_value_max_bytes":           ConfigApplyPolicyHotReload,
	"storage.kv_total_limit_mb":            ConfigApplyPolicyHotReload,
	"storage.file_max_bytes":               ConfigApplyPolicyHotReload,
	"storage.plugin_workdir_soft_limit_mb": ConfigApplyPolicyHotReload,

	"data.audit_logs_retention_days":     ConfigApplyPolicyRestartRequired,
	"data.event_records_retention_days":  ConfigApplyPolicyRestartRequired,
	"data.download_cache_retention_days": ConfigApplyPolicyRestartRequired,

	"log.level":                 ConfigApplyPolicyHotReload,
	"log.retention_days":        ConfigApplyPolicyHotReload,
	"log.rate_limit_per_plugin": ConfigApplyPolicyHotReload,

	"message.rate_limit_per_plugin":   ConfigApplyPolicyHotReload,
	"message.rate_limit_per_target":   ConfigApplyPolicyHotReload,
	"message.circuit_breaker_seconds": ConfigApplyPolicyHotReload,

	"user.command_rate_limit":  ConfigApplyPolicyHotReload,
	"user.cooldown_reply":      ConfigApplyPolicyHotReload,
	"group.command_rate_limit": ConfigApplyPolicyHotReload,

	"adapter.connect_timeout_seconds":   ConfigApplyPolicyAdapterReload,
	"adapter.reconnect_initial_seconds": ConfigApplyPolicyAdapterReload,
	"adapter.reconnect_multiplier":      ConfigApplyPolicyAdapterReload,
	"adapter.reconnect_max_seconds":     ConfigApplyPolicyAdapterReload,
	"adapter.reconnect_jitter_ratio":    ConfigApplyPolicyAdapterReload,

	"http.timeout_seconds":     ConfigApplyPolicyHotReload,
	"http.max_retries":         ConfigApplyPolicyHotReload,
	"http.allow_private_hosts": ConfigApplyPolicyHotReload,

	"web.exposure_mode":    ConfigApplyPolicyRestartRequired,
	"web.setup_local_only": ConfigApplyPolicyRestartRequired,

	"backup.default_consistency": ConfigApplyPolicyRestartRequired,
}

func ConfigApplyPolicyForPath(path string) (ConfigApplyPolicy, bool) {
	policy, ok := configApplyPolicies[path]
	return policy, ok
}

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

func normalizeConfigEffectPaths(paths []string) []string {
	if len(paths) == 0 {
		return []string{}
	}

	normalized := append([]string(nil), paths...)
	slices.Sort(normalized)
	return slices.Compact(normalized)
}
