package permission

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
)

type Verdict struct {
	Allowed   bool
	Reason    string
	ErrorCode string
	Err       error
	// Scope identifies which target a blacklist/rate-limit rejection applies
	// to: "user" or "group". Empty for verdicts without a target scope.
	Scope string
}

const (
	ScopeUser  = "user"
	ScopeGroup = "group"
)

type CheckerConfig struct {
	SuperAdmins  []string
	DefaultLevel string // "super_admin", "group_admin", "everyone"
}

type CommandInfo struct {
	Permission string // "super_admin", "group_admin", "everyone"
}

type Checker struct {
	cfg                CheckerConfig
	whitelistRepo      EntryRepository
	whitelistStateRepo WhitelistStateRepository
	blacklistRepo      EntryRepository
	cooldown           *CooldownTracker
}

func NewChecker(cfg CheckerConfig, whitelistRepo EntryRepository, whitelistStateRepo WhitelistStateRepository, blacklistRepo EntryRepository, cooldown *CooldownTracker) *Checker {
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
func (c *Checker) Check(ctx context.Context, scope chatevent.IdentityScope, actorID, actorRole, groupID string, cmd *CommandInfo) Verdict {

	// 1. Super admin bypass - skip all other checks.
	if scope.SourceProtocol == "onebot11" && slices.Contains(c.cfg.SuperAdmins, actorID) {
		return Verdict{Allowed: true}
	}

	skipBlacklist := false
	if cmd != nil && c.whitelistStateRepo != nil {
		enabled, err := c.whitelistStateRepo.Enabled(ctx)
		if err != nil {
			return unavailableVerdict(err)
		}
		if enabled {
			matched, err := c.matchesWhitelist(ctx, scope, actorID, groupID)
			if err != nil {
				return unavailableVerdict(err)
			}
			if !matched {
				return Verdict{Allowed: false, Reason: "发送者不在白名单中", ErrorCode: errorcodes.PermissionNotWhitelisted}
			}
			skipBlacklist = true
		}
	}

	// 2. Blacklist check.
	if !skipBlacklist && c.blacklistRepo != nil {
		blocked, err := c.blacklistRepo.Contains(ctx, scope, "user", actorID)
		if err != nil {
			return unavailableVerdict(err)
		}
		if blocked {
			return Verdict{Allowed: false, Reason: "用户在黑名单中", ErrorCode: errorcodes.PermissionBlacklisted, Scope: ScopeUser}
		}
		if groupID != "" {
			blocked, err := c.blacklistRepo.Contains(ctx, scope, "group", groupID)
			if err != nil {
				return unavailableVerdict(err)
			}
			if blocked {
				return Verdict{Allowed: false, Reason: "群在黑名单中", ErrorCode: errorcodes.PermissionBlacklisted, Scope: ScopeGroup}
			}
		}
	}

	// 3. Command permission level check.
	if cmd != nil && cmd.Permission != "" && cmd.Permission != "everyone" {
		if !hasPermissionLevel(actorRole, cmd.Permission) {
			return Verdict{Allowed: false, Reason: "权限等级不足", ErrorCode: errorcodes.PermissionDenied}
		}
	}

	// 4. Cooldown / rate limit check.
	if c.cooldown != nil && cmd != nil {
		userKey := "user:" + scope.Key("user", actorID)
		if !c.cooldown.Allow(userKey) {
			return Verdict{Allowed: false, Reason: "用户命令触发频率限制", ErrorCode: errorcodes.PlatformUserRateLimited, Scope: ScopeUser}
		}
		if groupID != "" {
			groupKey := "group:" + scope.Key("group", groupID)
			if !c.cooldown.Allow(groupKey) {
				return Verdict{Allowed: false, Reason: "群命令触发频率限制", ErrorCode: errorcodes.PlatformRateLimited, Scope: ScopeGroup}
			}
		}
	}

	return Verdict{Allowed: true}
}

func unavailableVerdict(err error) Verdict {
	return Verdict{Reason: "暂时无法确认权限，本次操作未执行", ErrorCode: errorcodes.PermissionUnavailable, Err: err}
}

func (c *Checker) matchesWhitelist(ctx context.Context, scope chatevent.IdentityScope, actorID, groupID string) (bool, error) {
	if c.whitelistRepo == nil {
		return false, errors.New("whitelist repository is unavailable")
	}

	matchedUser, err := c.whitelistRepo.Contains(ctx, scope, "user", actorID)
	if err != nil || matchedUser {
		return matchedUser, err
	}

	if groupID == "" {
		return false, nil
	}

	return c.whitelistRepo.Contains(ctx, scope, "group", groupID)
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

type CooldownTracker struct {
	userLimit  config.RateLimit
	groupLimit config.RateLimit
	mu         sync.Mutex
	windows    map[string]*slidingWindow
}

type slidingWindow struct {
	timestamps []time.Time
	limit      config.RateLimit
}

func NewCooldownTracker(userLimit, groupLimit config.RateLimit) *CooldownTracker {
	return &CooldownTracker{
		userLimit:  userLimit,
		groupLimit: groupLimit,
		windows:    make(map[string]*slidingWindow),
	}
}

func (t *CooldownTracker) Allow(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	window, ok := t.windows[key]
	if !ok {
		limit := t.limitForKey(key)
		window = &slidingWindow{limit: limit}
		t.windows[key] = window
	}

	cutoff := now.Add(-window.limit.Window)
	valid := 0
	for _, timestamp := range window.timestamps {
		if timestamp.After(cutoff) {
			window.timestamps[valid] = timestamp
			valid++
		}
	}
	window.timestamps = window.timestamps[:valid]

	if len(window.timestamps) >= window.limit.Count {
		return false
	}

	window.timestamps = append(window.timestamps, now)
	return true
}

func (t *CooldownTracker) limitForKey(key string) config.RateLimit {
	if strings.HasPrefix(key, "group:") {
		return t.groupLimit
	}
	return t.userLimit
}

func (t *CooldownTracker) Cleanup() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for key, window := range t.windows {
		cutoff := now.Add(-window.limit.Window)
		hasValid := false
		for _, timestamp := range window.timestamps {
			if timestamp.After(cutoff) {
				hasValid = true
				break
			}
		}
		if !hasValid {
			delete(t.windows, key)
		}
	}
}
