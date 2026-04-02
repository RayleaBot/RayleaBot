package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

const currentSchemaVersion = "2"

func CurrentSchemaVersion() string {
	return currentSchemaVersion
}

func loadCanonicalDocument(configPath, schemaPath string) (map[string]any, Config, error) {
	defaultDoc, err := ensureDefaultTemplate(configPath)
	if err != nil {
		return nil, Config{}, err
	}

	rawUser, userExists, err := readYAMLDocument(configPath)
	if err != nil {
		return nil, Config{}, fmt.Errorf("read config %s: %w", configPath, err)
	}

	userDoc := map[string]any{}
	if userExists {
		userDoc, err = canonicalizeDocument(rawUser)
		if err != nil {
			return nil, Config{}, fmt.Errorf("normalize config document %s: %w", configPath, err)
		}
	}

	document := mergeDocuments(defaultDoc, userDoc)
	if err := validateDocument(schemaPath, document); err != nil {
		return nil, Config{}, fmt.Errorf("config validation failed for %s against %s: %w", configPath, schemaPath, err)
	}

	cfg, err := decodeTypedConfig(document)
	if err != nil {
		return nil, Config{}, fmt.Errorf("decode typed config %s: %w", configPath, err)
	}

	shouldPersist := !userExists || !reflect.DeepEqual(rawUser, document)
	if shouldPersist {
		if err := writeCanonicalDocument(configPath, document); err != nil {
			return nil, Config{}, err
		}
	}

	return document, cfg, nil
}

func ensureDefaultTemplate(configPath string) (map[string]any, error) {
	defaultPath := defaultTemplatePath(configPath)
	rawDefault, exists, err := readYAMLDocument(defaultPath)
	if err != nil {
		return nil, fmt.Errorf("read default config %s: %w", defaultPath, err)
	}

	document := defaultDocument()
	if exists {
		canonicalDefault, err := canonicalizeDocument(rawDefault)
		if err != nil {
			return nil, fmt.Errorf("normalize default config %s: %w", defaultPath, err)
		}
		document = mergeDocuments(document, canonicalDefault)
	}

	if !exists || !reflect.DeepEqual(rawDefault, document) {
		if err := writeCanonicalDocument(defaultPath, document); err != nil {
			return nil, err
		}
	}

	return document, nil
}

func defaultTemplatePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "default.yaml")
}

func readYAMLDocument(path string) (map[string]any, bool, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(bytes, &raw); err != nil {
		return nil, true, fmt.Errorf("parse yaml %s: %w", path, err)
	}
	return raw, true, nil
}

func writeCanonicalDocument(path string, document map[string]any) error {
	yamlBytes, err := yaml.Marshal(document)
	if err != nil {
		return fmt.Errorf("marshal config yaml %s: %w", path, err)
	}
	return writeAtomic(path, yamlBytes, 0o644)
}

func canonicalizeDocument(raw map[string]any) (map[string]any, error) {
	normalized, err := normalizeDocument(raw)
	if err != nil {
		return nil, err
	}

	document, ok := normalized.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("normalized document is not an object")
	}
	if isLegacyDocument(document) {
		return legacyToCanonical(document), nil
	}

	cloned := CloneDocument(document)
	if cloned == nil {
		cloned = map[string]any{}
	}
	if version := strings.TrimSpace(stringValue(cloned["schema_version"])); version == "" {
		cloned["schema_version"] = currentSchemaVersion
	}
	return cloned, nil
}

func isLegacyDocument(document map[string]any) bool {
	if document == nil {
		return false
	}
	if _, ok := document["logging"]; ok {
		return true
	}
	if _, ok := document["auth"]; ok {
		return true
	}
	if _, ok := document["retention"]; ok {
		return true
	}
	if _, ok := document["cooldown"]; ok {
		return true
	}
	if runtimeSection := section(document, "runtime"); runtimeSection != nil {
		if _, ok := runtimeSection["scheduler_timezone"]; ok {
			return true
		}
	}
	if onebotSection := section(document, "onebot"); onebotSection != nil {
		if _, ok := onebotSection["connect_timeout_seconds"]; ok {
			return true
		}
	}
	return strings.TrimSpace(stringValue(document["schema_version"])) == "1"
}

func legacyToCanonical(document map[string]any) map[string]any {
	canonical := map[string]any{
		"schema_version": currentSchemaVersion,
	}
	copySectionIfPresent(canonical, "server", document, "server")
	copySectionIfPresent(canonical, "database", document, "database")
	copySectionIfPresent(canonical, "storage", document, "storage")
	copySectionIfPresent(canonical, "http", document, "http")
	copySectionIfPresent(canonical, "render", document, "render")
	copySectionIfPresent(canonical, "web", document, "web")
	copySectionIfPresent(canonical, "backup", document, "backup")
	copySectionIfPresent(canonical, "command", document, "command")

	if onebot := section(document, "onebot"); onebot != nil {
		canonical["onebot"] = map[string]any{
			"ws_url":       onebot["ws_url"],
			"access_token": onebot["access_token"],
		}
		canonical["adapter"] = map[string]any{
			"connect_timeout_seconds":   onebot["connect_timeout_seconds"],
			"reconnect_initial_seconds": onebot["reconnect_initial_seconds"],
			"reconnect_multiplier":      onebot["reconnect_multiplier"],
			"reconnect_max_seconds":     onebot["reconnect_max_seconds"],
			"reconnect_jitter_ratio":    onebot["reconnect_jitter_ratio"],
		}
	}

	if auth := section(document, "auth"); auth != nil {
		canonical["admin"] = map[string]any{
			"super_admins":              auth["super_admins"],
			"session_ttl_days":          auth["session_ttl_days"],
			"sliding_renewal":           auth["sliding_renewal"],
			"max_sessions":              auth["max_sessions"],
			"login_fail_limit":          auth["login_fail_limit"],
			"login_fail_window_seconds": auth["login_fail_window_seconds"],
		}
		canonical["permission"] = map[string]any{
			"default_level":           auth["default_level"],
			"auto_grant_capabilities": auth["auto_grant_capabilities"],
		}
	}

	if runtime := section(document, "runtime"); runtime != nil {
		canonical["scheduler"] = map[string]any{
			"timezone": runtime["scheduler_timezone"],
		}
		canonical["runtime"] = CloneDocument(runtime)
		delete(canonical["runtime"].(map[string]any), "scheduler_timezone")
	}

	if logging := section(document, "logging"); logging != nil {
		canonical["log"] = CloneDocument(logging)
	}
	if retention := section(document, "retention"); retention != nil {
		canonical["data"] = CloneDocument(retention)
	}
	if cooldown := section(document, "cooldown"); cooldown != nil {
		canonical["user"] = map[string]any{
			"command_rate_limit": cooldown["user_command_rate_limit"],
			"cooldown_reply":     cooldown["cooldown_reply"],
		}
		canonical["group"] = map[string]any{
			"command_rate_limit": cooldown["group_command_rate_limit"],
		}
	}

	return canonical
}

func copySectionIfPresent(target map[string]any, targetKey string, source map[string]any, sourceKey string) {
	if value := section(source, sourceKey); value != nil {
		target[targetKey] = CloneDocument(value)
	}
}

func section(document map[string]any, key string) map[string]any {
	value, ok := document[key]
	if !ok {
		return nil
	}
	typed, _ := value.(map[string]any)
	return typed
}

func mergeDocuments(base, overlay map[string]any) map[string]any {
	result := CloneDocument(base)
	if result == nil {
		result = map[string]any{}
	}
	for key, value := range overlay {
		targetSection, targetIsMap := result[key].(map[string]any)
		sourceSection, sourceIsMap := value.(map[string]any)
		if targetIsMap && sourceIsMap {
			result[key] = mergeDocuments(targetSection, sourceSection)
			continue
		}
		result[key] = cloneValue(value)
	}
	return result
}

func cloneValue(value any) any {
	bytes, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var cloned any
	if err := json.Unmarshal(bytes, &cloned); err != nil {
		return value
	}
	return cloned
}

func decodeTypedConfig(document map[string]any) (Config, error) {
	var cfg Config
	jsonBytes, err := json.Marshal(document)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(jsonBytes, &cfg); err != nil {
		return cfg, err
	}
	cfg.hydrateCompatibility()
	return cfg, nil
}

func canonicalDocumentFromTyped(cfg Config) map[string]any {
	cfg.hydrateCompatibility()
	return map[string]any{
		"schema_version": currentSchemaVersion,
		"server": map[string]any{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		},
		"onebot": map[string]any{
			"ws_url":       cfg.OneBot.WSURL,
			"access_token": cfg.OneBot.AccessToken,
		},
		"database": map[string]any{
			"engine": cfg.Database.Engine,
			"path":   cfg.Database.Path,
		},
		"command": map[string]any{
			"prefixes": configCommandPrefixes(cfg),
		},
		"admin": map[string]any{
			"super_admins":              append([]string{}, configSuperAdmins(cfg)...),
			"session_ttl_days":          configSessionTTLDays(cfg),
			"sliding_renewal":           configSlidingRenewal(cfg),
			"max_sessions":              configMaxSessions(cfg),
			"login_fail_limit":          configLoginFailLimit(cfg),
			"login_fail_window_seconds": configLoginFailWindowSeconds(cfg),
		},
		"permission": map[string]any{
			"default_level":           configDefaultLevel(cfg),
			"auto_grant_capabilities": append([]string{}, configAutoGrantCapabilities(cfg)...),
		},
		"render": map[string]any{
			"worker_count":               cfg.Render.WorkerCount,
			"browser_args":               append([]string{}, cfg.Render.BrowserArgs...),
			"browser_path":               cfg.Render.BrowserPath,
			"timeout_seconds":            cfg.Render.TimeoutSeconds,
			"queue_wait_timeout_seconds": cfg.Render.QueueWaitTimeoutSeconds,
			"queue_max_length":           cfg.Render.QueueMaxLength,
		},
		"scheduler": map[string]any{
			"timezone": configSchedulerTimezone(cfg),
		},
		"runtime": map[string]any{
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
		"storage": map[string]any{
			"kv_value_max_bytes":           cfg.Storage.KVValueMaxBytes,
			"kv_total_limit_mb":            cfg.Storage.KVTotalLimitMB,
			"file_max_bytes":               cfg.Storage.FileMaxBytes,
			"plugin_workdir_soft_limit_mb": cfg.Storage.PluginWorkDirMB,
		},
		"data": map[string]any{
			"audit_logs_retention_days":     configAuditLogsRetentionDays(cfg),
			"event_records_retention_days":  configEventRecordsRetentionDays(cfg),
			"download_cache_retention_days": configDownloadCacheRetentionDays(cfg),
		},
		"log": map[string]any{
			"level":                 configLogLevel(cfg),
			"retention_days":        configLogRetentionDays(cfg),
			"rate_limit_per_plugin": configLogRateLimit(cfg),
		},
		"message": map[string]any{
			"rate_limit_per_plugin":   configMessageRateLimitPerPlugin(cfg),
			"rate_limit_per_target":   configMessageRateLimitPerTarget(cfg),
			"circuit_breaker_seconds": configMessageCircuitBreakerSeconds(cfg),
		},
		"user": map[string]any{
			"command_rate_limit": configUserCommandRateLimit(cfg),
			"cooldown_reply":     configCooldownReply(cfg),
		},
		"group": map[string]any{
			"command_rate_limit": configGroupCommandRateLimit(cfg),
		},
		"adapter": map[string]any{
			"connect_timeout_seconds":   configAdapterConnectTimeout(cfg),
			"reconnect_initial_seconds": configAdapterReconnectInitial(cfg),
			"reconnect_multiplier":      configAdapterReconnectMultiplier(cfg),
			"reconnect_max_seconds":     configAdapterReconnectMax(cfg),
			"reconnect_jitter_ratio":    configAdapterReconnectJitter(cfg),
		},
		"http": map[string]any{
			"timeout_seconds":     cfg.HTTP.TimeoutSeconds,
			"max_retries":         cfg.HTTP.MaxRetries,
			"allow_private_hosts": append([]string{}, cfg.HTTP.AllowPrivateHosts...),
		},
		"web": map[string]any{
			"exposure_mode":    cfg.Web.ExposureMode,
			"setup_local_only": cfg.Web.SetupLocalOnly,
		},
		"backup": map[string]any{
			"default_consistency": cfg.Backup.DefaultConsistency,
		},
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

func configSuperAdmins(cfg Config) []string {
	if len(cfg.Admin.SuperAdmins) > 0 {
		return cfg.Admin.SuperAdmins
	}
	return cfg.Auth.SuperAdmins
}

func configDefaultLevel(cfg Config) string {
	if strings.TrimSpace(cfg.Permission.DefaultLevel) != "" {
		return cfg.Permission.DefaultLevel
	}
	return cfg.Auth.DefaultLevel
}

func configAutoGrantCapabilities(cfg Config) []string {
	if len(cfg.Permission.AutoGrantCapabilities) > 0 {
		return cfg.Permission.AutoGrantCapabilities
	}
	return cfg.Auth.AutoGrantCapabilities
}

func configSessionTTLDays(cfg Config) int {
	if cfg.Admin.SessionTTLDays > 0 {
		return cfg.Admin.SessionTTLDays
	}
	return cfg.Auth.SessionTTLDays
}

func configSlidingRenewal(cfg Config) bool {
	if cfg.Admin.SessionTTLDays > 0 {
		return cfg.Admin.SlidingRenewal
	}
	return cfg.Auth.SlidingRenewal
}

func configMaxSessions(cfg Config) int {
	if cfg.Admin.MaxSessions > 0 {
		return cfg.Admin.MaxSessions
	}
	return cfg.Auth.MaxSessions
}

func configLoginFailLimit(cfg Config) int {
	if cfg.Admin.LoginFailLimit > 0 {
		return cfg.Admin.LoginFailLimit
	}
	return cfg.Auth.LoginFailLimit
}

func configLoginFailWindowSeconds(cfg Config) int {
	if cfg.Admin.LoginFailWindowSecs > 0 {
		return cfg.Admin.LoginFailWindowSecs
	}
	return cfg.Auth.LoginFailWindowSecs
}

func configSchedulerTimezone(cfg Config) string {
	if cfg.Scheduler.Timezone != "" {
		return cfg.Scheduler.Timezone
	}
	return cfg.Runtime.SchedulerTimezone
}

func configAuditLogsRetentionDays(cfg Config) int {
	if cfg.Data.AuditLogsRetentionDays > 0 {
		return cfg.Data.AuditLogsRetentionDays
	}
	return cfg.Retention.AuditLogsRetentionDays
}

func configEventRecordsRetentionDays(cfg Config) int {
	if cfg.Data.EventRecordsRetentionDays > 0 {
		return cfg.Data.EventRecordsRetentionDays
	}
	return cfg.Retention.EventRecordsRetentionDays
}

func configDownloadCacheRetentionDays(cfg Config) int {
	if cfg.Data.DownloadCacheRetentionDays > 0 {
		return cfg.Data.DownloadCacheRetentionDays
	}
	return cfg.Retention.DownloadCacheRetentionDays
}

func configLogLevel(cfg Config) string {
	if cfg.Log.Level != "" {
		return cfg.Log.Level
	}
	return cfg.Logging.Level
}

func configLogRetentionDays(cfg Config) int {
	if cfg.Log.RetentionDays > 0 {
		return cfg.Log.RetentionDays
	}
	return cfg.Logging.RetentionDays
}

func configLogRateLimit(cfg Config) string {
	if cfg.Log.RateLimitPerPlugin != "" {
		return cfg.Log.RateLimitPerPlugin
	}
	return cfg.Logging.RateLimitPerPlugin
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
	if cfg.Cooldown != nil && cfg.Cooldown.UserCommandRateLimit != "" {
		return cfg.Cooldown.UserCommandRateLimit
	}
	return "10/60s"
}

func configGroupCommandRateLimit(cfg Config) string {
	if cfg.Group.CommandRateLimit != "" {
		return cfg.Group.CommandRateLimit
	}
	if cfg.Cooldown != nil && cfg.Cooldown.GroupCommandRateLimit != "" {
		return cfg.Cooldown.GroupCommandRateLimit
	}
	return "30/60s"
}

func configCooldownReply(cfg Config) bool {
	if cfg.User.CommandRateLimit != "" || cfg.User.CooldownReply {
		return cfg.User.CooldownReply
	}
	if cfg.Cooldown != nil {
		return cfg.Cooldown.CooldownReply
	}
	return true
}

func configAdapterConnectTimeout(cfg Config) int {
	if cfg.Adapter.ConnectTimeoutSeconds > 0 {
		return cfg.Adapter.ConnectTimeoutSeconds
	}
	return cfg.OneBot.ConnectTimeoutSeconds
}

func configAdapterReconnectInitial(cfg Config) int {
	if cfg.Adapter.ReconnectInitialSeconds > 0 {
		return cfg.Adapter.ReconnectInitialSeconds
	}
	return cfg.OneBot.ReconnectInitialSeconds
}

func configAdapterReconnectMultiplier(cfg Config) float64 {
	if cfg.Adapter.ReconnectMultiplier > 0 {
		return cfg.Adapter.ReconnectMultiplier
	}
	return cfg.OneBot.ReconnectMultiplier
}

func configAdapterReconnectMax(cfg Config) int {
	if cfg.Adapter.ReconnectMaxSeconds > 0 {
		return cfg.Adapter.ReconnectMaxSeconds
	}
	return cfg.OneBot.ReconnectMaxSeconds
}

func configAdapterReconnectJitter(cfg Config) float64 {
	if cfg.Adapter.ReconnectJitterRatio > 0 {
		return cfg.Adapter.ReconnectJitterRatio
	}
	return cfg.OneBot.ReconnectJitterRatio
}

func defaultDocument() map[string]any {
	return map[string]any{
		"schema_version": currentSchemaVersion,
		"server": map[string]any{
			"host": "127.0.0.1",
			"port": 8080,
		},
		"onebot": map[string]any{
			"ws_url":       "",
			"access_token": "",
		},
		"database": map[string]any{
			"engine": "sqlite",
			"path":   "data/rayleabot.db",
		},
		"command": map[string]any{
			"prefixes": []string{"/"},
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
			"default_level":           "everyone",
			"auto_grant_capabilities": []string{},
		},
		"render": map[string]any{
			"worker_count":               1,
			"browser_args":               []string{"--disable-gpu"},
			"browser_path":               "",
			"timeout_seconds":            30,
			"queue_wait_timeout_seconds": 15,
			"queue_max_length":           32,
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
			"command_rate_limit": "10/60s",
			"cooldown_reply":     true,
		},
		"group": map[string]any{
			"command_rate_limit": "30/60s",
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

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
