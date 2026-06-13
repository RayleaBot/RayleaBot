package config

func configServerDocument(cfg Config) map[string]any {
	return map[string]any{
		"host": cfg.Server.Host,
		"port": cfg.Server.Port,
	}
}

func configOneBotDocument(cfg Config) map[string]any {
	return map[string]any{
		"reverse_ws": oneBotTransportConfigDocument(cfg.OneBot.ReverseWS),
		"forward_ws": oneBotTransportConfigDocument(cfg.OneBot.ForwardWS),
		"http_api":   oneBotTransportConfigDocument(cfg.OneBot.HTTPAPI),
		"webhook":    oneBotTransportConfigDocument(cfg.OneBot.Webhook),
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
		"sliding_renewal":           cfg.Admin.SlidingRenewal,
		"max_sessions":              cfg.Admin.MaxSessions,
		"login_fail_limit":          cfg.Admin.LoginFailLimit,
		"login_fail_window_seconds": cfg.Admin.LoginFailWindowSecs,
	}
}

func configPermissionDocument(cfg Config) map[string]any {
	return map[string]any{
		"default_level":           cfg.Permission.DefaultLevel,
		"auto_grant_capabilities": append([]string{}, cfg.Permission.AutoGrantCapabilities...),
	}
}
