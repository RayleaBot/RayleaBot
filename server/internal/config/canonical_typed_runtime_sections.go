package config

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
