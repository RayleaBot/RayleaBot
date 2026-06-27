package permission

import (
	"context"
	"slices"
)

type Verdict struct {
	Allowed   bool
	Reason    string
	ErrorCode string
}

type CheckerConfig struct {
	SuperAdmins  []string
	DefaultLevel string // "super_admin", "group_admin", "everyone"
}

type CommandInfo struct {
	Permission string // "super_admin", "group_admin", "everyone"
}

type Checker struct {
	cfg                CheckerConfig
	whitelistRepo      WhitelistRepository
	whitelistStateRepo WhitelistStateRepository
	blacklistRepo      BlacklistRepository
	cooldown           *CooldownTracker
}

func NewChecker(cfg CheckerConfig, whitelistRepo WhitelistRepository, whitelistStateRepo WhitelistStateRepository, blacklistRepo BlacklistRepository, cooldown *CooldownTracker) *Checker {
	return &Checker{
		cfg:                cfg,
		whitelistRepo:      whitelistRepo,
		whitelistStateRepo: whitelistStateRepo,
		blacklistRepo:      blacklistRepo,
		cooldown:           cooldown,
	}
}

// Check runs the permission check sequence:
// super_admin bypass -> whitelist command admission -> blacklist -> permission level -> cooldown.
// actorID is the sender, actorRole is "owner"/"admin"/"member"/""
// groupID is the conversation group ID (empty for private messages)
// cmd is non-nil only when the message is a parsed command
func (c *Checker) Check(ctx context.Context, actorID, actorRole, groupID string, cmd *CommandInfo) Verdict {
	if c == nil {
		return Verdict{Allowed: true}
	}

	// 1. Super admin bypass - skip all other checks.
	if slices.Contains(c.cfg.SuperAdmins, actorID) {
		return Verdict{Allowed: true}
	}

	skipBlacklist := false
	if cmd != nil && c.whitelistStateRepo != nil {
		if enabled, err := c.whitelistStateRepo.Enabled(ctx); err == nil && enabled {
			if !c.matchesWhitelist(ctx, actorID, groupID) {
				return Verdict{Allowed: false, Reason: "发送者不在白名单中", ErrorCode: "permission.not_whitelisted"}
			}
			skipBlacklist = true
		}
	}

	// 2. Blacklist check.
	if !skipBlacklist && c.blacklistRepo != nil {
		if blocked, _ := c.blacklistRepo.IsBlacklisted(ctx, "user", actorID); blocked {
			return Verdict{Allowed: false, Reason: "用户在黑名单中", ErrorCode: "permission.blacklisted"}
		}
		if groupID != "" {
			if blocked, _ := c.blacklistRepo.IsBlacklisted(ctx, "group", groupID); blocked {
				return Verdict{Allowed: false, Reason: "群在黑名单中", ErrorCode: "permission.blacklisted"}
			}
		}
	}

	// 3. Command permission level check.
	if cmd != nil && cmd.Permission != "" && cmd.Permission != "everyone" {
		if !hasPermissionLevel(actorRole, cmd.Permission) {
			return Verdict{Allowed: false, Reason: "权限等级不足", ErrorCode: "permission.denied"}
		}
	}

	// 4. Cooldown / rate limit check.
	if c.cooldown != nil && cmd != nil {
		userKey := "user:" + actorID
		if !c.cooldown.Allow(userKey) {
			return Verdict{Allowed: false, Reason: "用户命令触发频率限制", ErrorCode: "platform.user_rate_limited"}
		}
		if groupID != "" {
			groupKey := "group:" + groupID
			if !c.cooldown.Allow(groupKey) {
				return Verdict{Allowed: false, Reason: "群命令触发频率限制", ErrorCode: "platform.rate_limited"}
			}
		}
	}

	return Verdict{Allowed: true}
}

func (c *Checker) matchesWhitelist(ctx context.Context, actorID, groupID string) bool {
	if c == nil || c.whitelistRepo == nil {
		return false
	}

	matchedUser, err := c.whitelistRepo.IsWhitelisted(ctx, "user", actorID)
	if err == nil && matchedUser {
		return true
	}

	if groupID == "" {
		return false
	}

	matchedGroup, err := c.whitelistRepo.IsWhitelisted(ctx, "group", groupID)
	return err == nil && matchedGroup
}

// hasPermissionLevel checks if actorRole meets the required permission level.
// Hierarchy: super_admin > group_admin (owner/admin) > everyone (member/"")
func hasPermissionLevel(actorRole, requiredLevel string) bool {
	roleRank := roleToRank(actorRole)
	requiredRank := levelToRank(requiredLevel)
	return roleRank >= requiredRank
}

func roleToRank(role string) int {
	switch role {
	case "owner":
		return 3
	case "admin":
		return 2
	case "member", "":
		return 1
	default:
		return 1
	}
}

func levelToRank(level string) int {
	switch level {
	case "super_admin":
		return 4
	case "group_admin":
		return 2
	case "everyone", "":
		return 1
	default:
		return 1
	}
}
