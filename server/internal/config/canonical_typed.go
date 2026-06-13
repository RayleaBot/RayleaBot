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
