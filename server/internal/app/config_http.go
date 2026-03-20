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
		_, _, err := internalconfig.SaveDocument(a.Summary.ConfigPath, a.Summary.SchemaPath, resolved)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, "platform.invalid_config", "配置校验失败", "errors.platform.invalid_config", nil)
			return
		}

		document, redactedFields := sanitizeConfigDocument(resolved)
		writeAuthJSON(w, http.StatusOK, configUpdateResponse{
			Config:          document,
			RedactedFields:  redactedFields,
			RestartRequired: true,
		})
	}
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
