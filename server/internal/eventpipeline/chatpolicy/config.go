package chatpolicy

import (
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
)

type ConfigSnapshot struct {
	SuperAdmins           []string
	DefaultLevel          string
	UserCommandRateLimit  string
	GroupCommandRateLimit string
	CooldownReplyEnabled  bool
}

func newPermissionChecker(cfg config.Config, whitelistRepo permission.WhitelistRepository, whitelistState permission.WhitelistStateRepository, blacklistRepo permission.BlacklistRepository) *permission.Checker {
	settings := ResolveConfig(cfg)
	userLimit := parseCooldownRateLimitWithFallback(settings.UserCommandRateLimit, config.DefaultUserCommandRateLimit)
	groupLimit := parseCooldownRateLimitWithFallback(settings.GroupCommandRateLimit, config.DefaultGroupCommandRateLimit)

	return permission.NewChecker(permission.CheckerConfig{
		SuperAdmins:  append([]string(nil), settings.SuperAdmins...),
		DefaultLevel: settings.DefaultLevel,
	}, whitelistRepo, whitelistState, blacklistRepo, permission.NewCooldownTracker(userLimit, groupLimit))
}

func parseCooldownRateLimitWithFallback(raw, fallback string) permission.RateLimit {
	if limit, err := permission.ParseRateLimit(strings.TrimSpace(raw)); err == nil {
		return limit
	}
	return parseCooldownRateLimit(fallback)
}

func parseCooldownRateLimit(raw string) permission.RateLimit {
	limit, err := permission.ParseRateLimit(raw)
	if err == nil {
		return limit
	}
	return permission.RateLimit{Count: 1, Window: time.Minute}
}

func commandPermissionDefaultLevel(cfg config.Config) string {
	defaultLevel := strings.TrimSpace(ResolveConfig(cfg).DefaultLevel)
	switch defaultLevel {
	case "super_admin", "group_admin", "everyone":
		return defaultLevel
	default:
		return "everyone"
	}
}

func cooldownReplyEnabled(cfg config.Config) bool {
	return ResolveConfig(cfg).CooldownReplyEnabled
}

func ResolveConfig(cfg config.Config) ConfigSnapshot {
	settings := ConfigSnapshot{
		SuperAdmins:           append([]string(nil), cfg.Admin.SuperAdmins...),
		DefaultLevel:          strings.TrimSpace(cfg.Permission.DefaultLevel),
		UserCommandRateLimit:  strings.TrimSpace(cfg.User.CommandRateLimit),
		GroupCommandRateLimit: strings.TrimSpace(cfg.Group.CommandRateLimit),
		CooldownReplyEnabled:  cfg.User.CooldownReply,
	}

	if settings.UserCommandRateLimit == "" {
		settings.UserCommandRateLimit = config.DefaultUserCommandRateLimit
	}
	if settings.GroupCommandRateLimit == "" {
		settings.GroupCommandRateLimit = config.DefaultGroupCommandRateLimit
	}
	if settings.DefaultLevel == "" {
		settings.DefaultLevel = "everyone"
	}
	return settings
}
