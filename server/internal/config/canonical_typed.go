package config

import "strings"

func canonicalDocumentFromTyped(cfg Config) map[string]any {
	return map[string]any{
		"schema_version":   currentSchemaVersion,
		"server":           configServerDocument(cfg),
		"adapters":         configAdaptersDocument(cfg),
		"database":         configDatabaseDocument(cfg),
		"command":          configCommandDocument(cfg),
		"builtin_features": configBuiltinFeaturesDocument(cfg),
		"admin":            configAdminDocument(cfg),
		"permission":       configPermissionDocument(cfg),
		"render":           configRenderDocument(cfg),
		"scheduler":        configSchedulerDocument(cfg),
		"runtime":          configRuntimeDocument(cfg),
		"storage":          configStorageDocument(cfg),
		"data":             configDataDocument(cfg),
		"log":              configLogDocument(cfg),
		"message":          configMessageDocument(cfg),
		"user":             configUserDocument(cfg),
		"group":            configGroupDocument(cfg),
		"adapter":          configAdapterDocument(cfg),
	}
}

func CanonicalDocumentFromTyped(cfg Config) map[string]any {
	return canonicalDocumentFromTyped(cfg)
}

func configCommandPrefixes(cfg Config) []string {
	if cfg.Command != nil {
		return append([]string{}, cfg.Command.Prefixes...)
	}
	return []string{"/"}
}

func configBuiltinMenuCommands(cfg Config) []string {
	return append([]string{}, cfg.Builtin.Menu.Commands...)
}

func configBuiltinMenuPrefixes(cfg Config) []string {
	if len(cfg.Builtin.Menu.Prefixes) > 0 {
		return append([]string{}, cfg.Builtin.Menu.Prefixes...)
	}
	return []string{}
}

func configMessageRateLimitPerTarget(cfg Config) string {
	if cfg.Message.RateLimitPerTarget != "" {
		return cfg.Message.RateLimitPerTarget
	}
	return "5/5s"
}

func configUserCommandRateLimit(cfg Config) string {
	if cfg.User.CommandRateLimit != "" {
		return cfg.User.CommandRateLimit
	}
	return DefaultUserCommandRateLimit
}

func configGroupCommandRateLimit(cfg Config) string {
	if cfg.Group.CommandRateLimit != "" {
		return cfg.Group.CommandRateLimit
	}
	return DefaultGroupCommandRateLimit
}

func configRenderFooterTemplate(cfg Config) string {
	return cfg.Render.FooterTemplate
}

func configRenderDefaultOutput(cfg Config) string {
	switch strings.TrimSpace(strings.ToLower(cfg.Render.DefaultOutput)) {
	case "jpeg":
		return "jpeg"
	default:
		return DefaultRenderOutput
	}
}

func configRenderDeviceScalePercent(cfg Config) int {
	if cfg.Render.DeviceScalePercent >= 50 && cfg.Render.DeviceScalePercent <= 500 {
		return cfg.Render.DeviceScalePercent
	}
	return DefaultRenderDeviceScalePercent
}

func configServerDocument(cfg Config) map[string]any {
	return map[string]any{
		"host": cfg.Server.Host,
		"port": cfg.Server.Port,
	}
}

func configAdaptersDocument(cfg Config) []any {
	adapters := make([]any, 0, len(cfg.Adapters))
	for _, adapter := range cfg.Adapters {
		document := map[string]any{
			"id":      adapter.ID,
			"type":    adapter.Type,
			"enabled": adapter.Enabled,
		}
		if adapter.OneBot11 != nil {
			document["onebot11"] = oneBotSettingsDocument(*adapter.OneBot11)
		}
		if adapter.QQOfficial != nil {
			document["qqofficial"] = qqOfficialSettingsDocument(*adapter.QQOfficial)
		}
		adapters = append(adapters, document)
	}
	return adapters
}

func oneBotSettingsDocument(settings OneBotConfig) map[string]any {
	return map[string]any{
		"reverse_ws": oneBotTransportCompatDocument(settings.ReverseWS),
		"forward_ws": oneBotTransportCompatDocument(settings.ForwardWS),
		"http_api":   oneBotTransportConfigDocument(settings.HTTPAPI),
		"webhook":    oneBotTransportCompatDocument(settings.Webhook),
	}
}

func qqOfficialSettingsDocument(settings QQOfficialConfig) map[string]any {
	intents := settings.Intents
	if intents == nil {
		// The schema types intents as an array; a nil slice marshals to null.
		intents = []string{}
	}
	return map[string]any{
		"app_id":     settings.AppID,
		"app_secret": settings.AppSecret,
		"intents":    intents,
		"sandbox":    settings.Sandbox,
	}
}

func configDatabaseDocument(cfg Config) map[string]any {
	return map[string]any{
		"engine": cfg.Database.Engine,
		"path":   cfg.Database.Path,
	}
}

func configCommandDocument(cfg Config) map[string]any {
	return map[string]any{
		"prefixes": configCommandPrefixes(cfg),
	}
}

func configBuiltinFeaturesDocument(cfg Config) map[string]any {
	return map[string]any{
		"menu": map[string]any{
			"commands": configBuiltinMenuCommands(cfg),
			"prefixes": configBuiltinMenuPrefixes(cfg),
		},
	}
}

func configAdminDocument(cfg Config) map[string]any {
	return map[string]any{
		"super_admins":              append([]string{}, cfg.Admin.SuperAdmins...),
		"session_ttl_days":          cfg.Admin.SessionTTLDays,
		"session_absolute_ttl_days": cfg.Admin.SessionAbsoluteTTLDays,
		"sliding_renewal":           cfg.Admin.SlidingRenewal,
		"max_sessions":              cfg.Admin.MaxSessions,
		"login_fail_limit":          cfg.Admin.LoginFailLimit,
		"login_fail_window_seconds": cfg.Admin.LoginFailWindowSecs,
	}
}

func configPermissionDocument(cfg Config) map[string]any {
	return map[string]any{
		"default_level": cfg.Permission.DefaultLevel,
	}
}

func configMessageDocument(cfg Config) map[string]any {
	return map[string]any{
		"rate_limit_per_target": configMessageRateLimitPerTarget(cfg),
	}
}

func configUserDocument(cfg Config) map[string]any {
	return map[string]any{
		"command_rate_limit":  configUserCommandRateLimit(cfg),
		"cooldown_reply":      cfg.User.CooldownReply,
		"cooldown_reply_once": cfg.User.CooldownReplyOnce,
	}
}

func configGroupDocument(cfg Config) map[string]any {
	return map[string]any{
		"command_rate_limit": configGroupCommandRateLimit(cfg),
	}
}

func configAdapterDocument(cfg Config) map[string]any {
	return map[string]any{
		"connect_timeout_seconds":   cfg.Adapter.ConnectTimeoutSeconds,
		"reconnect_initial_seconds": cfg.Adapter.ReconnectInitialSeconds,
		"reconnect_multiplier":      cfg.Adapter.ReconnectMultiplier,
		"reconnect_max_seconds":     cfg.Adapter.ReconnectMaxSeconds,
		"reconnect_jitter_ratio":    cfg.Adapter.ReconnectJitterRatio,
	}
}

func configRenderDocument(cfg Config) map[string]any {
	return map[string]any{
		"worker_count":               cfg.Render.WorkerCount,
		"browser_args":               append([]string{}, cfg.Render.BrowserArgs...),
		"browser_path":               cfg.Render.BrowserPath,
		"default_output":             configRenderDefaultOutput(cfg),
		"device_scale_percent":       configRenderDeviceScalePercent(cfg),
		"timeout_seconds":            cfg.Render.TimeoutSeconds,
		"queue_wait_timeout_seconds": cfg.Render.QueueWaitTimeoutSeconds,
		"queue_max_length":           cfg.Render.QueueMaxLength,
		"footer_template":            configRenderFooterTemplate(cfg),
	}
}

func configSchedulerDocument(cfg Config) map[string]any {
	return map[string]any{
		"timezone": cfg.Scheduler.Timezone,
	}
}

func configRuntimeDocument(cfg Config) map[string]any {
	return map[string]any{
		"plugin_init_timeout_seconds":           cfg.Runtime.PluginInitTimeoutSeconds,
		"plugin_event_timeout_seconds":          cfg.Runtime.PluginEventTimeoutSeconds,
		"max_pending_events_per_plugin":         cfg.Runtime.MaxPendingEventsPerPlugin,
		"max_pending_control_events_per_plugin": cfg.Runtime.MaxPendingControlEvents,
		"stderr_rate_limit_bytes_per_second":    cfg.Runtime.StderrRateLimitBytesPerSec,
		"max_concurrent_tasks_per_plugin":       cfg.Runtime.MaxConcurrentTasksPerPlugin,
		"plugin_detached_event_timeout_seconds": cfg.Runtime.PluginDetachedEventTimeoutSeconds,
		"max_detached_events_per_plugin":        cfg.Runtime.MaxDetachedEventsPerPlugin,
		"crash_backoff_initial_seconds":         cfg.Runtime.CrashBackoffInitialSeconds,
		"crash_backoff_max_seconds":             cfg.Runtime.CrashBackoffMaxSeconds,
		"shutdown_grace_seconds":                cfg.Runtime.ShutdownGraceSeconds,
		"ipc_message_max_bytes":                 cfg.Runtime.IPCMessageMaxBytes,
	}
}

func configStorageDocument(cfg Config) map[string]any {
	return map[string]any{
		"kv_value_max_bytes": cfg.Storage.KVValueMaxBytes,
		"kv_total_limit_mb":  cfg.Storage.KVTotalLimitMB,
	}
}

func configDataDocument(cfg Config) map[string]any {
	return map[string]any{
		"download_cache_retention_days": cfg.Data.DownloadCacheRetentionDays,
	}
}

func configLogDocument(cfg Config) map[string]any {
	return map[string]any{
		"level":          cfg.Log.Level,
		"retention_days": cfg.Log.RetentionDays,
	}
}
