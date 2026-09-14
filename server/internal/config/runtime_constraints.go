package config

import (
	"fmt"
	"net"
	"strings"
)

func validateRuntimeConstraints(cfg Config) error {
	for _, field := range []struct{ name, value string }{
		{"user.command_rate_limit", cfg.User.CommandRateLimit},
		{"group.command_rate_limit", cfg.Group.CommandRateLimit},
		{"message.rate_limit_per_target", cfg.Message.RateLimitPerTarget},
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
	return nil
}

func validateBindHost(raw string) error {
	host := strings.TrimSpace(strings.Trim(raw, "[]"))
	if strings.EqualFold(host, "localhost") || net.ParseIP(host) != nil {
		return nil
	}
	return fmt.Errorf("server.host must be an IP address or localhost")
}
