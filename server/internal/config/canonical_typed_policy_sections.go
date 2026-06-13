package config

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
		"timeout_seconds":     cfg.HTTP.TimeoutSeconds,
		"max_retries":         cfg.HTTP.MaxRetries,
		"allow_private_hosts": append([]string{}, cfg.HTTP.AllowPrivateHosts...),
	}
}

func configWebDocument(cfg Config) map[string]any {
	return map[string]any{
		"exposure_mode":    cfg.Web.ExposureMode,
		"setup_local_only": cfg.Web.SetupLocalOnly,
	}
}

func configBackupDocument(cfg Config) map[string]any {
	return map[string]any{
		"default_consistency": cfg.Backup.DefaultConsistency,
	}
}
