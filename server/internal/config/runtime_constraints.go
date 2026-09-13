package config

import (
	"fmt"
	"net"
	"net/url"
	"slices"
	"strings"
)

func validateRuntimeConstraints(cfg Config) error {
	for _, field := range []struct{ name, value string }{
		{"user.command_rate_limit", cfg.User.CommandRateLimit},
		{"group.command_rate_limit", cfg.Group.CommandRateLimit},
		{"message.rate_limit_per_plugin", cfg.Message.RateLimitPerPlugin},
		{"message.rate_limit_per_target", cfg.Message.RateLimitPerTarget},
		{"log.rate_limit_per_plugin", cfg.Log.RateLimitPerPlugin},
		{"runtime.ipc_action_burst_limit", cfg.Runtime.IPCActionBurstLimit},
	} {
		if _, err := ParseRateLimit(field.value); err != nil {
			return fmt.Errorf("%s: %w", field.name, err)
		}
	}
	if _, err := LoadTimezone(cfg.Scheduler.Timezone); err != nil {
		return err
	}
	if cfg.Admin.SessionAbsoluteTTLDays < cfg.Admin.SessionTTLDays {
		return fmt.Errorf("admin.session_absolute_ttl_days must be greater than or equal to admin.session_ttl_days")
	}

	return validateWebOptions(cfg)
}

func validateWebOptions(cfg Config) error {
	if err := validateBindHost(cfg.Server.Host); err != nil {
		return err
	}
	if template := strings.TrimSpace(cfg.Web.PluginUIOriginTemplate); template != "" {
		if !strings.Contains(template, "{plugin_host}") {
			return fmt.Errorf("web.plugin_ui_origin_template must contain {plugin_host}")
		}
		if !isStrictOrigin(strings.ReplaceAll(template, "{plugin_host}", "p-0123456789abcdef"), "http", "https") {
			return fmt.Errorf("web.plugin_ui_origin_template must render to an HTTP(S) origin")
		}
	}

	return nil
}

// isStrictOrigin reports whether raw is a bare origin (scheme and host only)
// using one of the given schemes.
func isStrictOrigin(raw string, schemes ...string) bool {
	origin, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || origin.Host == "" || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}
	return slices.Contains(schemes, origin.Scheme)
}

func validateBindHost(raw string) error {
	host := strings.TrimSpace(strings.Trim(raw, "[]"))
	if strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil {
		return nil
	}
	return fmt.Errorf("server.host must be an IP address or localhost")
}
