package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"gopkg.in/yaml.v3"

	"rayleabot/server/internal/schema"
)

type Config struct {
	SchemaVersion string          `json:"schema_version" yaml:"schema_version"`
	Server        ServerConfig    `json:"server" yaml:"server"`
	OneBot        OneBotConfig    `json:"onebot" yaml:"onebot"`
	Database      DatabaseConfig  `json:"database" yaml:"database"`
	Logging       LoggingConfig   `json:"logging" yaml:"logging"`
	Auth          AuthConfig      `json:"auth" yaml:"auth"`
	Runtime       RuntimeConfig   `json:"runtime" yaml:"runtime"`
	Render        RenderConfig    `json:"render" yaml:"render"`
	Web           WebConfig       `json:"web" yaml:"web"`
	Backup        BackupConfig    `json:"backup" yaml:"backup"`
	Retention     RetentionConfig `json:"retention" yaml:"retention"`
}

type ServerConfig struct {
	Host string `json:"host" yaml:"host"`
	Port int    `json:"port" yaml:"port"`
}

type OneBotConfig struct {
	WSURL                   string  `json:"ws_url" yaml:"ws_url"`
	AccessToken             string  `json:"access_token" yaml:"access_token"`
	ConnectTimeoutSeconds   int     `json:"connect_timeout_seconds" yaml:"connect_timeout_seconds"`
	ReconnectInitialSeconds int     `json:"reconnect_initial_seconds" yaml:"reconnect_initial_seconds"`
	ReconnectMultiplier     float64 `json:"reconnect_multiplier" yaml:"reconnect_multiplier"`
	ReconnectMaxSeconds     int     `json:"reconnect_max_seconds" yaml:"reconnect_max_seconds"`
	ReconnectJitterRatio    float64 `json:"reconnect_jitter_ratio" yaml:"reconnect_jitter_ratio"`
}

type DatabaseConfig struct {
	Engine string `json:"engine" yaml:"engine"`
	Path   string `json:"path" yaml:"path"`
}

type LoggingConfig struct {
	Level              string `json:"level" yaml:"level"`
	RetentionDays      int    `json:"retention_days" yaml:"retention_days"`
	RateLimitPerPlugin string `json:"rate_limit_per_plugin" yaml:"rate_limit_per_plugin"`
}

type AuthConfig struct {
	SuperAdmins           []string `json:"super_admins" yaml:"super_admins"`
	DefaultLevel          string   `json:"default_level" yaml:"default_level"`
	AutoGrantCapabilities []string `json:"auto_grant_capabilities" yaml:"auto_grant_capabilities"`
	SessionTTLDays        int      `json:"session_ttl_days" yaml:"session_ttl_days"`
	SlidingRenewal        bool     `json:"sliding_renewal" yaml:"sliding_renewal"`
	MaxSessions           int      `json:"max_sessions" yaml:"max_sessions"`
	LoginFailLimit        int      `json:"login_fail_limit" yaml:"login_fail_limit"`
	LoginFailWindowSecs   int      `json:"login_fail_window_seconds" yaml:"login_fail_window_seconds"`
}

type RuntimeConfig struct {
	SchedulerTimezone            string `json:"scheduler_timezone" yaml:"scheduler_timezone"`
	PluginInitTimeoutSeconds     int    `json:"plugin_init_timeout_seconds" yaml:"plugin_init_timeout_seconds"`
	PluginInitMaxTotalSeconds    int    `json:"plugin_init_max_total_seconds" yaml:"plugin_init_max_total_seconds"`
	PluginEventTimeoutSeconds    int    `json:"plugin_event_timeout_seconds" yaml:"plugin_event_timeout_seconds"`
	MaxPendingEventsPerPlugin    int    `json:"max_pending_events_per_plugin" yaml:"max_pending_events_per_plugin"`
	MaxPendingControlEvents      int    `json:"max_pending_control_events_per_plugin" yaml:"max_pending_control_events_per_plugin"`
	NodeMaxOldSpaceSizeMB        int    `json:"nodejs_max_old_space_size_mb" yaml:"nodejs_max_old_space_size_mb"`
	DependencyInstallTimeoutSecs int    `json:"dependency_install_timeout_seconds" yaml:"dependency_install_timeout_seconds"`
	MaxConcurrentDependencyInst  int    `json:"max_concurrent_dependency_installs" yaml:"max_concurrent_dependency_installs"`
	IPCPendingActionsMax         int    `json:"ipc_pending_actions_max" yaml:"ipc_pending_actions_max"`
	IPCActionBurstLimit          string `json:"ipc_action_burst_limit" yaml:"ipc_action_burst_limit"`
	StderrRateLimitBytesPerSec   int    `json:"stderr_rate_limit_bytes_per_second" yaml:"stderr_rate_limit_bytes_per_second"`
	MaxConcurrentTasksPerPlugin  int    `json:"max_concurrent_tasks_per_plugin" yaml:"max_concurrent_tasks_per_plugin"`
	CrashBackoffInitialSeconds   int    `json:"crash_backoff_initial_seconds" yaml:"crash_backoff_initial_seconds"`
	CrashBackoffMaxSeconds       int    `json:"crash_backoff_max_seconds" yaml:"crash_backoff_max_seconds"`
	ShutdownGraceSeconds         int    `json:"shutdown_grace_seconds" yaml:"shutdown_grace_seconds"`
	IPCMessageMaxBytes           int    `json:"ipc_message_max_bytes" yaml:"ipc_message_max_bytes"`
}

type RenderConfig struct {
	WorkerCount             int      `json:"worker_count" yaml:"worker_count"`
	BrowserArgs             []string `json:"browser_args" yaml:"browser_args"`
	BrowserPath             string   `json:"browser_path" yaml:"browser_path"`
	TimeoutSeconds          int      `json:"timeout_seconds" yaml:"timeout_seconds"`
	QueueWaitTimeoutSeconds int      `json:"queue_wait_timeout_seconds" yaml:"queue_wait_timeout_seconds"`
	QueueMaxLength          int      `json:"queue_max_length" yaml:"queue_max_length"`
}

type WebConfig struct {
	ExposureMode  string `json:"exposure_mode" yaml:"exposure_mode"`
	SetupLocalOnly bool  `json:"setup_local_only" yaml:"setup_local_only"`
}

type BackupConfig struct {
	DefaultConsistency string `json:"default_consistency" yaml:"default_consistency"`
}

type RetentionConfig struct {
	AuditLogsRetentionDays    int `json:"audit_logs_retention_days" yaml:"audit_logs_retention_days"`
	EventRecordsRetentionDays int `json:"event_records_retention_days" yaml:"event_records_retention_days"`
	DownloadCacheRetentionDays int `json:"download_cache_retention_days" yaml:"download_cache_retention_days"`
}

type Summary struct {
	ConfigPath       string
	SchemaPath       string
	ServerHost       string
	ServerPort       int
	DatabaseEngine   string
	DatabasePath     string
	WebExposureMode  string
	LoggingLevel     string
	SuperAdminCount  int
	OneBotConfigured bool
	OneBotEndpoint   string
}

func Load(configPath, schemaPath string) (Config, Summary, error) {
	var cfg Config

	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return cfg, Summary{}, fmt.Errorf("read config %s: %w", configPath, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(configBytes, &raw); err != nil {
		return cfg, Summary{}, fmt.Errorf("parse yaml %s: %w", configPath, err)
	}

	document, err := normalizeDocument(raw)
	if err != nil {
		return cfg, Summary{}, fmt.Errorf("normalize config document %s: %w", configPath, err)
	}

	if err := validateDocument(schemaPath, document); err != nil {
		return cfg, Summary{}, fmt.Errorf("config validation failed for %s against %s: %w", configPath, schemaPath, err)
	}

	jsonBytes, err := json.Marshal(document)
	if err != nil {
		return cfg, Summary{}, fmt.Errorf("marshal normalized config %s: %w", configPath, err)
	}

	if err := json.Unmarshal(jsonBytes, &cfg); err != nil {
		return cfg, Summary{}, fmt.Errorf("decode typed config %s: %w", configPath, err)
	}

	return cfg, buildSummary(configPath, schemaPath, cfg), nil
}

func normalizeDocument(raw map[string]any) (any, error) {
	jsonBytes, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	var document any
	if err := json.Unmarshal(jsonBytes, &document); err != nil {
		return nil, err
	}

	return document, nil
}

func validateDocument(schemaPath string, document any) error {
	validator, err := schema.Compile(schemaPath)
	if err != nil {
		return err
	}

	if err := validator.Validate(document); err != nil {
		return err
	}

	return nil
}

func buildSummary(configPath, schemaPath string, cfg Config) Summary {
	return Summary{
		ConfigPath:       configPath,
		SchemaPath:       schemaPath,
		ServerHost:       cfg.Server.Host,
		ServerPort:       cfg.Server.Port,
		DatabaseEngine:   cfg.Database.Engine,
		DatabasePath:     cfg.Database.Path,
		WebExposureMode:  cfg.Web.ExposureMode,
		LoggingLevel:     cfg.Logging.Level,
		SuperAdminCount:  len(cfg.Auth.SuperAdmins),
		OneBotConfigured: cfg.OneBot.WSURL != "",
		OneBotEndpoint:   sanitizeOneBotEndpoint(cfg.OneBot.WSURL),
	}
}

func sanitizeOneBotEndpoint(raw string) string {
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return parsed.Scheme + "://" + parsed.Host
}
