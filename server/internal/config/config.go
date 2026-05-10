package config

import (
	"encoding/json"
	"net/url"

	"github.com/RayleaBot/RayleaBot/server/internal/schema"
)

type Config struct {
	SchemaVersion string           `json:"schema_version" yaml:"schema_version"`
	Server        ServerConfig     `json:"server" yaml:"server"`
	OneBot        OneBotConfig     `json:"onebot" yaml:"onebot"`
	Database      DatabaseConfig   `json:"database" yaml:"database"`
	Command       *CommandConfig   `json:"command" yaml:"command"`
	Builtin       BuiltinConfig    `json:"builtin_features" yaml:"builtin_features"`
	Admin         AdminConfig      `json:"admin" yaml:"admin"`
	Permission    PermissionConfig `json:"permission" yaml:"permission"`
	Render        RenderConfig     `json:"render" yaml:"render"`
	Scheduler     SchedulerConfig  `json:"scheduler" yaml:"scheduler"`
	Runtime       RuntimeConfig    `json:"runtime" yaml:"runtime"`
	Storage       StorageConfig    `json:"storage" yaml:"storage"`
	Data          DataConfig       `json:"data" yaml:"data"`
	Log           LogConfig        `json:"log" yaml:"log"`
	Message       MessageConfig    `json:"message" yaml:"message"`
	User          UserConfig       `json:"user" yaml:"user"`
	Group         GroupConfig      `json:"group" yaml:"group"`
	Adapter       AdapterConfig    `json:"adapter" yaml:"adapter"`
	HTTP          HTTPConfig       `json:"http" yaml:"http"`
	Web           WebConfig        `json:"web" yaml:"web"`
	Backup        BackupConfig     `json:"backup" yaml:"backup"`

	// Legacy compatibility fields retained as internal mirrors while the
	// runtime and tests finish moving to the planning-aligned config shape.
	Logging   LoggingConfig   `json:"logging,omitempty" yaml:"logging,omitempty"`
	Auth      AuthConfig      `json:"auth,omitempty" yaml:"auth,omitempty"`
	Retention RetentionConfig `json:"retention,omitempty" yaml:"retention,omitempty"`
	Cooldown  *CooldownConfig `json:"cooldown,omitempty" yaml:"cooldown,omitempty"`
}

type CommandConfig struct {
	Prefixes []string `json:"prefixes" yaml:"prefixes"`
}

type BuiltinConfig struct {
	Menu BuiltinMenuConfig `json:"menu" yaml:"menu"`
}

type BuiltinMenuConfig struct {
	Commands []string `json:"commands" yaml:"commands"`
	Prefixes []string `json:"prefixes" yaml:"prefixes"`
}

type AdminConfig struct {
	SuperAdmins         []string `json:"super_admins" yaml:"super_admins"`
	SessionTTLDays      int      `json:"session_ttl_days" yaml:"session_ttl_days"`
	SlidingRenewal      bool     `json:"sliding_renewal" yaml:"sliding_renewal"`
	MaxSessions         int      `json:"max_sessions" yaml:"max_sessions"`
	LoginFailLimit      int      `json:"login_fail_limit" yaml:"login_fail_limit"`
	LoginFailWindowSecs int      `json:"login_fail_window_seconds" yaml:"login_fail_window_seconds"`
}

type PermissionConfig struct {
	DefaultLevel          string   `json:"default_level" yaml:"default_level"`
	AutoGrantCapabilities []string `json:"auto_grant_capabilities" yaml:"auto_grant_capabilities"`
}

type SchedulerConfig struct {
	Timezone string `json:"timezone" yaml:"timezone"`
}

type DataConfig struct {
	AuditLogsRetentionDays     int `json:"audit_logs_retention_days" yaml:"audit_logs_retention_days"`
	EventRecordsRetentionDays  int `json:"event_records_retention_days" yaml:"event_records_retention_days"`
	DownloadCacheRetentionDays int `json:"download_cache_retention_days" yaml:"download_cache_retention_days"`
}

type LogConfig struct {
	Level              string `json:"level" yaml:"level"`
	RetentionDays      int    `json:"retention_days" yaml:"retention_days"`
	RateLimitPerPlugin string `json:"rate_limit_per_plugin" yaml:"rate_limit_per_plugin"`
}

type MessageConfig struct {
	RateLimitPerPlugin    string `json:"rate_limit_per_plugin" yaml:"rate_limit_per_plugin"`
	RateLimitPerTarget    string `json:"rate_limit_per_target" yaml:"rate_limit_per_target"`
	CircuitBreakerSeconds int    `json:"circuit_breaker_seconds" yaml:"circuit_breaker_seconds"`
}

type UserConfig struct {
	CommandRateLimit string `json:"command_rate_limit" yaml:"command_rate_limit"`
	CooldownReply    bool   `json:"cooldown_reply" yaml:"cooldown_reply"`
}

type GroupConfig struct {
	CommandRateLimit string `json:"command_rate_limit" yaml:"command_rate_limit"`
}

type AdapterConfig struct {
	ConnectTimeoutSeconds   int     `json:"connect_timeout_seconds" yaml:"connect_timeout_seconds"`
	ReconnectInitialSeconds int     `json:"reconnect_initial_seconds" yaml:"reconnect_initial_seconds"`
	ReconnectMultiplier     float64 `json:"reconnect_multiplier" yaml:"reconnect_multiplier"`
	ReconnectMaxSeconds     int     `json:"reconnect_max_seconds" yaml:"reconnect_max_seconds"`
	ReconnectJitterRatio    float64 `json:"reconnect_jitter_ratio" yaml:"reconnect_jitter_ratio"`
}

type OneBotTransportConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	URL         string `json:"url" yaml:"url"`
	AccessToken string `json:"access_token" yaml:"access_token"`
}

type CooldownConfig struct {
	UserCommandRateLimit  string `json:"user_command_rate_limit" yaml:"user_command_rate_limit"`
	GroupCommandRateLimit string `json:"group_command_rate_limit" yaml:"group_command_rate_limit"`
	CooldownReply         bool   `json:"cooldown_reply" yaml:"cooldown_reply"`
}

type ServerConfig struct {
	Host string `json:"host" yaml:"host"`
	Port int    `json:"port" yaml:"port"`
}

type OneBotConfig struct {
	Provider                string                `json:"provider" yaml:"provider"`
	ReverseWS               OneBotTransportConfig `json:"reverse_ws" yaml:"reverse_ws"`
	ForwardWS               OneBotTransportConfig `json:"forward_ws" yaml:"forward_ws"`
	HTTPAPI                 OneBotTransportConfig `json:"http_api" yaml:"http_api"`
	Webhook                 OneBotTransportConfig `json:"webhook" yaml:"webhook"`
	ConnectTimeoutSeconds   int                   `json:"connect_timeout_seconds,omitempty" yaml:"connect_timeout_seconds,omitempty"`
	ReconnectInitialSeconds int                   `json:"reconnect_initial_seconds,omitempty" yaml:"reconnect_initial_seconds,omitempty"`
	ReconnectMultiplier     float64               `json:"reconnect_multiplier,omitempty" yaml:"reconnect_multiplier,omitempty"`
	ReconnectMaxSeconds     int                   `json:"reconnect_max_seconds,omitempty" yaml:"reconnect_max_seconds,omitempty"`
	ReconnectJitterRatio    float64               `json:"reconnect_jitter_ratio,omitempty" yaml:"reconnect_jitter_ratio,omitempty"`

	// Legacy migration inputs retained for in-memory compatibility only.
	WSURL string                `json:"-" yaml:"-"`
	SSE   OneBotTransportConfig `json:"-" yaml:"-"`
}

type DatabaseConfig struct {
	Engine string `json:"engine" yaml:"engine"`
	Path   string `json:"path" yaml:"path"`
}

type StorageConfig struct {
	KVValueMaxBytes int `json:"kv_value_max_bytes" yaml:"kv_value_max_bytes"`
	KVTotalLimitMB  int `json:"kv_total_limit_mb" yaml:"kv_total_limit_mb"`
	FileMaxBytes    int `json:"file_max_bytes" yaml:"file_max_bytes"`
	PluginWorkDirMB int `json:"plugin_workdir_soft_limit_mb" yaml:"plugin_workdir_soft_limit_mb"`
}

type HTTPConfig struct {
	TimeoutSeconds    int      `json:"timeout_seconds" yaml:"timeout_seconds"`
	MaxRetries        int      `json:"max_retries" yaml:"max_retries"`
	AllowPrivateHosts []string `json:"allow_private_hosts" yaml:"allow_private_hosts"`
}

type LoggingConfig = LogConfig

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
	SchedulerTimezone            string `json:"scheduler_timezone,omitempty" yaml:"scheduler_timezone,omitempty"`
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
	FooterTemplate          string   `json:"footer_template" yaml:"footer_template"`
}

type WebConfig struct {
	ExposureMode   string `json:"exposure_mode" yaml:"exposure_mode"`
	SetupLocalOnly bool   `json:"setup_local_only" yaml:"setup_local_only"`
}

type BackupConfig struct {
	DefaultConsistency string `json:"default_consistency" yaml:"default_consistency"`
}

type RetentionConfig = DataConfig

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

	document, cfg, err := loadCanonicalDocument(configPath, schemaPath)
	if err != nil {
		return cfg, Summary{}, err
	}

	return cfg, buildSummary(configPath, schemaPath, cfg, document), nil
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

func buildSummary(configPath, schemaPath string, cfg Config, _ map[string]any) Summary {
	endpoint := firstConfiguredOneBotEndpoint(cfg.OneBot)
	return Summary{
		ConfigPath:       configPath,
		SchemaPath:       schemaPath,
		ServerHost:       cfg.Server.Host,
		ServerPort:       cfg.Server.Port,
		DatabaseEngine:   cfg.Database.Engine,
		DatabasePath:     cfg.Database.Path,
		WebExposureMode:  cfg.Web.ExposureMode,
		LoggingLevel:     configLogLevel(cfg),
		SuperAdminCount:  len(configSuperAdmins(cfg)),
		OneBotConfigured: endpoint != "",
		OneBotEndpoint:   sanitizeOneBotEndpoint(endpoint),
	}
}

func (cfg *Config) hydrateCompatibility() {
	if cfg == nil {
		return
	}

	if cfg.Command == nil {
		cfg.Command = &CommandConfig{Prefixes: []string{"/"}}
	}
	if len(cfg.Builtin.Menu.Commands) == 0 {
		cfg.Builtin.Menu.Commands = []string{"help", "帮助"}
	}

	if len(cfg.Admin.SuperAdmins) == 0 && len(cfg.Auth.SuperAdmins) > 0 {
		cfg.Admin.SuperAdmins = append([]string(nil), cfg.Auth.SuperAdmins...)
	}
	if len(cfg.Permission.AutoGrantCapabilities) == 0 && len(cfg.Auth.AutoGrantCapabilities) > 0 {
		cfg.Permission.AutoGrantCapabilities = append([]string(nil), cfg.Auth.AutoGrantCapabilities...)
	}
	if cfg.Permission.DefaultLevel == "" && cfg.Auth.DefaultLevel != "" {
		cfg.Permission.DefaultLevel = cfg.Auth.DefaultLevel
	}
	if cfg.Admin.SessionTTLDays == 0 && cfg.Auth.SessionTTLDays > 0 {
		cfg.Admin.SessionTTLDays = cfg.Auth.SessionTTLDays
		cfg.Admin.SlidingRenewal = cfg.Auth.SlidingRenewal
		cfg.Admin.MaxSessions = cfg.Auth.MaxSessions
		cfg.Admin.LoginFailLimit = cfg.Auth.LoginFailLimit
		cfg.Admin.LoginFailWindowSecs = cfg.Auth.LoginFailWindowSecs
	}

	if cfg.Log.Level == "" && cfg.Logging.Level != "" {
		cfg.Log = cfg.Logging
	}
	if cfg.Data.AuditLogsRetentionDays == 0 && cfg.Retention.AuditLogsRetentionDays > 0 {
		cfg.Data = cfg.Retention
	}
	if cfg.Scheduler.Timezone == "" && cfg.Runtime.SchedulerTimezone != "" {
		cfg.Scheduler.Timezone = cfg.Runtime.SchedulerTimezone
	}
	if cfg.User.CommandRateLimit == "" && cfg.Cooldown != nil {
		cfg.User.CommandRateLimit = cfg.Cooldown.UserCommandRateLimit
		cfg.User.CooldownReply = cfg.Cooldown.CooldownReply
		cfg.Group.CommandRateLimit = cfg.Cooldown.GroupCommandRateLimit
	}
	if cfg.Adapter.ConnectTimeoutSeconds == 0 && cfg.OneBot.ConnectTimeoutSeconds > 0 {
		cfg.Adapter.ConnectTimeoutSeconds = cfg.OneBot.ConnectTimeoutSeconds
		cfg.Adapter.ReconnectInitialSeconds = cfg.OneBot.ReconnectInitialSeconds
		cfg.Adapter.ReconnectMultiplier = cfg.OneBot.ReconnectMultiplier
		cfg.Adapter.ReconnectMaxSeconds = cfg.OneBot.ReconnectMaxSeconds
		cfg.Adapter.ReconnectJitterRatio = cfg.OneBot.ReconnectJitterRatio
	}
	if cfg.OneBot.Provider == "" {
		cfg.OneBot.Provider = "standard"
	}
	if cfg.OneBot.WSURL == "" && cfg.OneBot.ForwardWS.URL != "" {
		cfg.OneBot.WSURL = cfg.OneBot.ForwardWS.URL
	}
	if cfg.OneBot.ForwardWS.URL == "" && cfg.OneBot.WSURL != "" {
		cfg.OneBot.ForwardWS.URL = cfg.OneBot.WSURL
		cfg.OneBot.ForwardWS.Enabled = true
	}

	if len(cfg.Auth.SuperAdmins) == 0 && len(cfg.Admin.SuperAdmins) > 0 {
		cfg.Auth.SuperAdmins = append([]string(nil), cfg.Admin.SuperAdmins...)
	}
	if cfg.Auth.DefaultLevel == "" && cfg.Permission.DefaultLevel != "" {
		cfg.Auth.DefaultLevel = cfg.Permission.DefaultLevel
	}
	if len(cfg.Auth.AutoGrantCapabilities) == 0 && len(cfg.Permission.AutoGrantCapabilities) > 0 {
		cfg.Auth.AutoGrantCapabilities = append([]string(nil), cfg.Permission.AutoGrantCapabilities...)
	}
	if cfg.Auth.SessionTTLDays == 0 && cfg.Admin.SessionTTLDays > 0 {
		cfg.Auth.SessionTTLDays = cfg.Admin.SessionTTLDays
		cfg.Auth.SlidingRenewal = cfg.Admin.SlidingRenewal
		cfg.Auth.MaxSessions = cfg.Admin.MaxSessions
		cfg.Auth.LoginFailLimit = cfg.Admin.LoginFailLimit
		cfg.Auth.LoginFailWindowSecs = cfg.Admin.LoginFailWindowSecs
	}
	if cfg.Logging.Level == "" && cfg.Log.Level != "" {
		cfg.Logging = cfg.Log
	}
	if cfg.Retention.AuditLogsRetentionDays == 0 && cfg.Data.AuditLogsRetentionDays > 0 {
		cfg.Retention = cfg.Data
	}
	if cfg.Runtime.SchedulerTimezone == "" && cfg.Scheduler.Timezone != "" {
		cfg.Runtime.SchedulerTimezone = cfg.Scheduler.Timezone
	}
	if cfg.Cooldown == nil && (cfg.User.CommandRateLimit != "" || cfg.Group.CommandRateLimit != "") {
		cfg.Cooldown = &CooldownConfig{
			UserCommandRateLimit:  cfg.User.CommandRateLimit,
			GroupCommandRateLimit: cfg.Group.CommandRateLimit,
			CooldownReply:         cfg.User.CooldownReply,
		}
	}
	if cfg.OneBot.ConnectTimeoutSeconds == 0 && cfg.Adapter.ConnectTimeoutSeconds > 0 {
		cfg.OneBot.ConnectTimeoutSeconds = cfg.Adapter.ConnectTimeoutSeconds
		cfg.OneBot.ReconnectInitialSeconds = cfg.Adapter.ReconnectInitialSeconds
		cfg.OneBot.ReconnectMultiplier = cfg.Adapter.ReconnectMultiplier
		cfg.OneBot.ReconnectMaxSeconds = cfg.Adapter.ReconnectMaxSeconds
		cfg.OneBot.ReconnectJitterRatio = cfg.Adapter.ReconnectJitterRatio
	}
	if cfg.OneBot.WSURL == "" && cfg.OneBot.ForwardWS.URL != "" {
		cfg.OneBot.WSURL = cfg.OneBot.ForwardWS.URL
	}
	if cfg.OneBot.ForwardWS.URL == "" && cfg.OneBot.WSURL != "" {
		cfg.OneBot.ForwardWS.URL = cfg.OneBot.WSURL
		cfg.OneBot.ForwardWS.Enabled = true
	}
	if cfg.Render.FooterTemplate == "" {
		cfg.Render.FooterTemplate = DefaultRenderFooterTemplate
	}
}

func firstConfiguredOneBotEndpoint(cfg OneBotConfig) string {
	for _, endpoint := range []string{
		cfg.ForwardWS.URL,
		cfg.ReverseWS.URL,
		cfg.HTTPAPI.URL,
		cfg.Webhook.URL,
		cfg.WSURL,
	} {
		if endpoint != "" {
			return endpoint
		}
	}
	return ""
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
