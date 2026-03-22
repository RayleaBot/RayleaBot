package app

import (
	"net/http"

	internalconfig "rayleabot/server/internal/config"
)

const redactedConfigValue = "__REDACTED__"

type configResponse struct {
	Config         map[string]any `json:"config"`
	RedactedFields []string       `json:"redacted_fields,omitempty"`
}

type configUpdateResponse struct {
	Config          map[string]any `json:"config"`
	RedactedFields  []string       `json:"redacted_fields,omitempty"`
	RestartRequired bool           `json:"restart_required"`
}

func (a *App) handleConfigGet() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		document, redactedFields := sanitizeConfigDocument(configDocumentFromTyped(a.Config))
		writeAuthJSON(w, http.StatusOK, configResponse{
			Config:         document,
			RedactedFields: redactedFields,
		})
	}
}

func (a *App) handleConfigPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := decodeStrictJSON(r, &request); err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		resolved := resolveRedactedConfigValues(request, a.Config)
		newCfg, _, err := internalconfig.SaveDocument(a.Summary.ConfigPath, a.Summary.SchemaPath, resolved)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, "platform.invalid_config", "配置校验失败", "errors.platform.invalid_config", nil)
			return
		}

		restartRequired := applyHotReloadableFields(a, newCfg)

		document, redactedFields := sanitizeConfigDocument(resolved)
		writeAuthJSON(w, http.StatusOK, configUpdateResponse{
			Config:          document,
			RedactedFields:  redactedFields,
			RestartRequired: restartRequired,
		})
	}
}

// applyHotReloadableFields compares the new config with the current config,
// applies fields that can take effect immediately, and returns true if any
// non-hot-reloadable field has changed (requiring a restart).
func applyHotReloadableFields(a *App, newCfg internalconfig.Config) bool {
	oldCfg := a.Config
	restartRequired := false

	// logging.level — immediate effect via LevelController.
	if newCfg.Logging.Level != oldCfg.Logging.Level {
		if a.LogLevel != nil {
			if err := a.LogLevel.SetLevel(newCfg.Logging.Level); err == nil {
				a.Logger.Info("log level changed",
					"component", "config",
					"old_level", oldCfg.Logging.Level,
					"new_level", newCfg.Logging.Level,
				)
			}
		}
	}

	// Fields that require a restart when changed.
	if newCfg.Server.Host != oldCfg.Server.Host ||
		newCfg.Server.Port != oldCfg.Server.Port {
		restartRequired = true
	}
	if newCfg.Database.Engine != oldCfg.Database.Engine ||
		newCfg.Database.Path != oldCfg.Database.Path {
		restartRequired = true
	}
	if newCfg.OneBot.WSURL != oldCfg.OneBot.WSURL ||
		newCfg.OneBot.AccessToken != oldCfg.OneBot.AccessToken ||
		newCfg.OneBot.ConnectTimeoutSeconds != oldCfg.OneBot.ConnectTimeoutSeconds ||
		newCfg.OneBot.ReconnectInitialSeconds != oldCfg.OneBot.ReconnectInitialSeconds ||
		newCfg.OneBot.ReconnectMultiplier != oldCfg.OneBot.ReconnectMultiplier ||
		newCfg.OneBot.ReconnectMaxSeconds != oldCfg.OneBot.ReconnectMaxSeconds ||
		newCfg.OneBot.ReconnectJitterRatio != oldCfg.OneBot.ReconnectJitterRatio {
		restartRequired = true
	}
	if newCfg.Auth.SessionTTLDays != oldCfg.Auth.SessionTTLDays ||
		newCfg.Auth.SlidingRenewal != oldCfg.Auth.SlidingRenewal ||
		newCfg.Auth.MaxSessions != oldCfg.Auth.MaxSessions {
		restartRequired = true
	}
	if newCfg.Web.ExposureMode != oldCfg.Web.ExposureMode ||
		newCfg.Web.SetupLocalOnly != oldCfg.Web.SetupLocalOnly {
		restartRequired = true
	}
	if newCfg.Render.WorkerCount != oldCfg.Render.WorkerCount ||
		newCfg.Render.BrowserPath != oldCfg.Render.BrowserPath {
		restartRequired = true
	}

	// Update in-memory config to reflect the saved state.
	a.Config = newCfg
	a.commandParser = newCommandParser(newCfg)
	a.permissionChecker = newPermissionChecker(newCfg, a.blacklistRepo)

	return restartRequired
}

func configDocumentFromTyped(cfg internalconfig.Config) map[string]any {
	document := map[string]any{
		"schema_version": cfg.SchemaVersion,
		"server": map[string]any{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		},
		"onebot": map[string]any{
			"ws_url":                    cfg.OneBot.WSURL,
			"access_token":              cfg.OneBot.AccessToken,
			"connect_timeout_seconds":   cfg.OneBot.ConnectTimeoutSeconds,
			"reconnect_initial_seconds": cfg.OneBot.ReconnectInitialSeconds,
			"reconnect_multiplier":      cfg.OneBot.ReconnectMultiplier,
			"reconnect_max_seconds":     cfg.OneBot.ReconnectMaxSeconds,
			"reconnect_jitter_ratio":    cfg.OneBot.ReconnectJitterRatio,
		},
		"database": map[string]any{
			"engine": cfg.Database.Engine,
			"path":   cfg.Database.Path,
		},
		"logging": map[string]any{
			"level":                 cfg.Logging.Level,
			"retention_days":        cfg.Logging.RetentionDays,
			"rate_limit_per_plugin": cfg.Logging.RateLimitPerPlugin,
		},
		"auth": map[string]any{
			"super_admins":              append([]string{}, cfg.Auth.SuperAdmins...),
			"default_level":             cfg.Auth.DefaultLevel,
			"auto_grant_capabilities":   append([]string{}, cfg.Auth.AutoGrantCapabilities...),
			"session_ttl_days":          cfg.Auth.SessionTTLDays,
			"sliding_renewal":           cfg.Auth.SlidingRenewal,
			"max_sessions":              cfg.Auth.MaxSessions,
			"login_fail_limit":          cfg.Auth.LoginFailLimit,
			"login_fail_window_seconds": cfg.Auth.LoginFailWindowSecs,
		},
		"command": map[string]any{
			"prefixes": sanitizeCommandPrefixes(commandPrefixes(cfg)),
		},
		"cooldown": map[string]any{
			"user_command_rate_limit":  cooldownUserLimit(cfg),
			"group_command_rate_limit": cooldownGroupLimit(cfg),
			"cooldown_reply":           cooldownReplyEnabled(cfg),
		},
		"runtime": map[string]any{
			"scheduler_timezone":                    cfg.Runtime.SchedulerTimezone,
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
		},
		"render": map[string]any{
			"worker_count":               cfg.Render.WorkerCount,
			"browser_args":               append([]string{}, cfg.Render.BrowserArgs...),
			"browser_path":               cfg.Render.BrowserPath,
			"timeout_seconds":            cfg.Render.TimeoutSeconds,
			"queue_wait_timeout_seconds": cfg.Render.QueueWaitTimeoutSeconds,
			"queue_max_length":           cfg.Render.QueueMaxLength,
		},
		"web": map[string]any{
			"exposure_mode":    cfg.Web.ExposureMode,
			"setup_local_only": cfg.Web.SetupLocalOnly,
		},
		"backup": map[string]any{
			"default_consistency": cfg.Backup.DefaultConsistency,
		},
		"retention": map[string]any{
			"audit_logs_retention_days":     cfg.Retention.AuditLogsRetentionDays,
			"event_records_retention_days":  cfg.Retention.EventRecordsRetentionDays,
			"download_cache_retention_days": cfg.Retention.DownloadCacheRetentionDays,
		},
	}

	return document
}

func commandPrefixes(cfg internalconfig.Config) []string {
	if cfg.Command == nil || len(cfg.Command.Prefixes) == 0 {
		return []string{"/"}
	}
	return append([]string(nil), cfg.Command.Prefixes...)
}

func cooldownUserLimit(cfg internalconfig.Config) string {
	if cfg.Cooldown == nil || cfg.Cooldown.UserCommandRateLimit == "" {
		return defaultUserCommandRateLimit
	}
	return cfg.Cooldown.UserCommandRateLimit
}

func cooldownGroupLimit(cfg internalconfig.Config) string {
	if cfg.Cooldown == nil || cfg.Cooldown.GroupCommandRateLimit == "" {
		return defaultGroupCommandRateLimit
	}
	return cfg.Cooldown.GroupCommandRateLimit
}

func sanitizeConfigDocument(document map[string]any) (map[string]any, []string) {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil, nil
	}

	redactedFields := make([]string, 0, 1)
	onebotSection, ok := cloned["onebot"].(map[string]any)
	if !ok {
		return cloned, redactedFields
	}

	accessToken, ok := onebotSection["access_token"].(string)
	if ok && accessToken != "" {
		onebotSection["access_token"] = redactedConfigValue
		redactedFields = append(redactedFields, "onebot.access_token")
	}

	return cloned, redactedFields
}

func resolveRedactedConfigValues(document map[string]any, current internalconfig.Config) map[string]any {
	cloned := internalconfig.CloneDocument(document)
	if cloned == nil {
		return nil
	}

	onebotSection, ok := cloned["onebot"].(map[string]any)
	if !ok {
		return cloned
	}

	accessToken, ok := onebotSection["access_token"].(string)
	if ok && accessToken == redactedConfigValue {
		onebotSection["access_token"] = current.OneBot.AccessToken
	}

	return cloned
}
