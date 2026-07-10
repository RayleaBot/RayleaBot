package config

import "strings"

func canonicalDocumentFromTyped(cfg Config) map[string]any {
	return map[string]any{
		"schema_version":   currentSchemaVersion,
		"server":           configServerDocument(cfg),
		"onebot":           configOneBotDocument(cfg),
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
		"http":             configHTTPDocument(cfg),
		"web":              configWebDocument(cfg),
		"backup":           configBackupDocument(cfg),
	}
}

func CanonicalDocumentFromTyped(cfg Config) map[string]any {
	return canonicalDocumentFromTyped(cfg)
}

func configCommandPrefixes(cfg Config) []string {
	if cfg.Command != nil && len(cfg.Command.Prefixes) > 0 {
		return append([]string{}, cfg.Command.Prefixes...)
	}
	return []string{"/"}
}

func configBuiltinMenuCommands(cfg Config) []string {
	if len(cfg.Builtin.Menu.Commands) > 0 {
		return append([]string{}, cfg.Builtin.Menu.Commands...)
	}
	return []string{"help", "帮助"}
}

func configBuiltinMenuPrefixes(cfg Config) []string {
	if len(cfg.Builtin.Menu.Prefixes) > 0 {
		return append([]string{}, cfg.Builtin.Menu.Prefixes...)
	}
	return []string{}
}

func configMessageRateLimitPerPlugin(cfg Config) string {
	if cfg.Message.RateLimitPerPlugin != "" {
		return cfg.Message.RateLimitPerPlugin
	}
	return "20/10s"
}

func configMessageRateLimitPerTarget(cfg Config) string {
	if cfg.Message.RateLimitPerTarget != "" {
		return cfg.Message.RateLimitPerTarget
	}
	return "5/5s"
}

func configMessageCircuitBreakerSeconds(cfg Config) int {
	if cfg.Message.CircuitBreakerSeconds > 0 {
		return cfg.Message.CircuitBreakerSeconds
	}
	return 30
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
	if strings.TrimSpace(cfg.Render.FooterTemplate) != "" {
		return cfg.Render.FooterTemplate
	}
	return DefaultRenderFooterTemplate
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

func configOneBotDocument(cfg Config) map[string]any {
	return map[string]any{
		"reverse_ws": oneBotTransportCompatDocument(cfg.OneBot.ReverseWS),
		"forward_ws": oneBotTransportCompatDocument(cfg.OneBot.ForwardWS),
		"http_api":   oneBotTransportConfigDocument(cfg.OneBot.HTTPAPI),
		"webhook":    oneBotTransportCompatDocument(cfg.OneBot.Webhook),
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
		"rate_limit_per_plugin":   configMessageRateLimitPerPlugin(cfg),
		"rate_limit_per_target":   configMessageRateLimitPerTarget(cfg),
		"circuit_breaker_seconds": configMessageCircuitBreakerSeconds(cfg),
	}
}

func configUserDocument(cfg Config) map[string]any {
	return map[string]any{
		"command_rate_limit": configUserCommandRateLimit(cfg),
		"cooldown_reply":     cfg.User.CooldownReply,
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

func configHTTPDocument(cfg Config) map[string]any {
	return map[string]any{
		"timeout_seconds":         cfg.HTTP.TimeoutSeconds,
		"max_retries":             cfg.HTTP.MaxRetries,
		"max_response_body_bytes": cfg.HTTP.MaxResponseBodyBytes,
		"allow_private_hosts":     append([]string{}, cfg.HTTP.AllowPrivateHosts...),
	}
}

func configWebDocument(cfg Config) map[string]any {
	return map[string]any{
		"exposure_mode":       cfg.Web.ExposureMode,
		"setup_local_only":    cfg.Web.SetupLocalOnly,
		"public_origin":       cfg.Web.PublicOrigin,
		"trusted_proxy_cidrs": append([]string{}, cfg.Web.TrustedProxyCIDRs...),
	}
}

func configBackupDocument(cfg Config) map[string]any {
	return map[string]any{
		"default_consistency": cfg.Backup.DefaultConsistency,
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
		"plugin_init_max_total_seconds":         cfg.Runtime.PluginInitMaxTotalSeconds,
		"plugin_event_timeout_seconds":          cfg.Runtime.PluginEventTimeoutSeconds,
		"max_pending_events_per_plugin":         cfg.Runtime.MaxPendingEventsPerPlugin,
		"max_pending_control_events_per_plugin": cfg.Runtime.MaxPendingControlEvents,
		"nodejs_max_old_space_size_mb":          cfg.Runtime.NodeMaxOldSpaceSizeMB,
		"dependency_install_timeout_seconds":    cfg.Runtime.DependencyInstallTimeoutSecs,
		"max_concurrent_dependency_installs":    cfg.Runtime.MaxConcurrentDependencyInst,
		"ipc_pending_actions_max":               cfg.Runtime.IPCPendingActionsMax,
		"ipc_action_burst_limit":                cfg.Runtime.IPCActionBurstLimit,
		"stderr_rate_limit_bytes_per_second":    cfg.Runtime.StderrRateLimitBytesPerSec,
		"max_concurrent_tasks_per_plugin":       cfg.Runtime.MaxConcurrentTasksPerPlugin,
		"crash_backoff_initial_seconds":         cfg.Runtime.CrashBackoffInitialSeconds,
		"crash_backoff_max_seconds":             cfg.Runtime.CrashBackoffMaxSeconds,
		"shutdown_grace_seconds":                cfg.Runtime.ShutdownGraceSeconds,
		"ipc_message_max_bytes":                 cfg.Runtime.IPCMessageMaxBytes,
	}
}

func configStorageDocument(cfg Config) map[string]any {
	return map[string]any{
		"kv_value_max_bytes":           cfg.Storage.KVValueMaxBytes,
		"kv_total_limit_mb":            cfg.Storage.KVTotalLimitMB,
		"file_max_bytes":               cfg.Storage.FileMaxBytes,
		"plugin_workdir_soft_limit_mb": cfg.Storage.PluginWorkDirMB,
	}
}

func configDataDocument(cfg Config) map[string]any {
	return map[string]any{
		"audit_logs_retention_days":     cfg.Data.AuditLogsRetentionDays,
		"event_records_retention_days":  cfg.Data.EventRecordsRetentionDays,
		"download_cache_retention_days": cfg.Data.DownloadCacheRetentionDays,
	}
}

func configLogDocument(cfg Config) map[string]any {
	return map[string]any{
		"level":                 cfg.Log.Level,
		"retention_days":        cfg.Log.RetentionDays,
		"rate_limit_per_plugin": cfg.Log.RateLimitPerPlugin,
	}
}
