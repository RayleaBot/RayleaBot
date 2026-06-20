package config

func defaultDocument() map[string]any {
	return map[string]any{
		"schema_version": currentSchemaVersion,
		"server": map[string]any{
			"host": "127.0.0.1",
			"port": 8080,
		},
		"onebot": map[string]any{
			"reverse_ws": oneBotTransportDocument(false, "", ""),
			"forward_ws": oneBotTransportDocument(false, "", ""),
			"http_api":   oneBotTransportDocument(false, "", ""),
			"webhook":    oneBotTransportDocument(false, "", ""),
		},
		"database": map[string]any{
			"engine": "sqlite",
			"path":   "data/rayleabot.db",
		},
		"command": map[string]any{
			"prefixes": []string{"/"},
		},
		"builtin_features": map[string]any{
			"menu": map[string]any{
				"commands": []string{"help", "帮助"},
				"prefixes": []string{},
			},
		},
		"admin": map[string]any{
			"super_admins":              []string{},
			"session_ttl_days":          7,
			"sliding_renewal":           true,
			"max_sessions":              3,
			"login_fail_limit":          5,
			"login_fail_window_seconds": 300,
		},
		"permission": map[string]any{
			"default_level": "everyone",
		},
		"render": map[string]any{
			"worker_count":               1,
			"browser_args":               []string{"--disable-gpu"},
			"browser_path":               "",
			"default_output":             DefaultRenderOutput,
			"device_scale_percent":       DefaultRenderDeviceScalePercent,
			"timeout_seconds":            30,
			"queue_wait_timeout_seconds": 15,
			"queue_max_length":           32,
			"footer_template":            DefaultRenderFooterTemplate,
		},
		"scheduler": map[string]any{
			"timezone": "",
		},
		"runtime": map[string]any{
			"plugin_init_timeout_seconds":           30,
			"plugin_init_max_total_seconds":         300,
			"plugin_event_timeout_seconds":          60,
			"max_pending_events_per_plugin":         16,
			"max_pending_control_events_per_plugin": 4,
			"nodejs_max_old_space_size_mb":          256,
			"dependency_install_timeout_seconds":    900,
			"max_concurrent_dependency_installs":    1,
			"ipc_pending_actions_max":               256,
			"ipc_action_burst_limit":                "100/1s",
			"stderr_rate_limit_bytes_per_second":    262144,
			"max_concurrent_tasks_per_plugin":       4,
			"crash_backoff_initial_seconds":         2,
			"crash_backoff_max_seconds":             60,
			"shutdown_grace_seconds":                10,
			"ipc_message_max_bytes":                 8388608,
		},
		"storage": map[string]any{
			"kv_value_max_bytes":           65536,
			"kv_total_limit_mb":            16,
			"file_max_bytes":               10485760,
			"plugin_workdir_soft_limit_mb": 256,
		},
		"data": map[string]any{
			"audit_logs_retention_days":     90,
			"event_records_retention_days":  7,
			"download_cache_retention_days": 15,
		},
		"log": map[string]any{
			"level":                 "info",
			"retention_days":        7,
			"rate_limit_per_plugin": "200/10s",
		},
		"message": map[string]any{
			"rate_limit_per_plugin":   "20/10s",
			"rate_limit_per_target":   "5/5s",
			"circuit_breaker_seconds": 30,
		},
		"user": map[string]any{
			"command_rate_limit": DefaultUserCommandRateLimit,
			"cooldown_reply":     DefaultCooldownReply,
		},
		"group": map[string]any{
			"command_rate_limit": DefaultGroupCommandRateLimit,
		},
		"adapter": map[string]any{
			"reconnect_initial_seconds": 2,
			"reconnect_multiplier":      2.0,
			"reconnect_max_seconds":     120,
			"reconnect_jitter_ratio":    0.2,
			"connect_timeout_seconds":   15,
		},
		"http": map[string]any{
			"timeout_seconds":     10,
			"max_retries":         2,
			"allow_private_hosts": []string{},
		},
		"web": map[string]any{
			"exposure_mode":    "localhost_only",
			"setup_local_only": true,
		},
		"backup": map[string]any{
			"default_consistency": "offline",
		},
	}
}
