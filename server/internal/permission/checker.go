package permission

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Verdict struct {
	Allowed   bool
	Reason    string
	ErrorCode string
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
			return Verdict{Allowed: false, Reason: "用户在黑名单中", ErrorCode: "permission.blacklisted", Scope: ScopeUser}
		}
		if groupID != "" {
			if blocked, _ := c.blacklistRepo.IsBlacklisted(ctx, "group", groupID); blocked {
				return Verdict{Allowed: false, Reason: "群在黑名单中", ErrorCode: "permission.blacklisted", Scope: ScopeGroup}
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
			return Verdict{Allowed: false, Reason: "用户命令触发频率限制", ErrorCode: "platform.user_rate_limited", Scope: ScopeUser}
		}
		if groupID != "" {
			groupKey := "group:" + groupID
			if !c.cooldown.Allow(groupKey) {
				return Verdict{Allowed: false, Reason: "群命令触发频率限制", ErrorCode: "platform.rate_limited", Scope: ScopeGroup}
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

var ErrGovernanceEntryNotFound = errors.New("governance entry not found")

type BlacklistEntry struct {
	ID        int64
	EntryType string
	TargetID  string
	Reason    string
	CreatedAt string
}

type BlacklistRepository interface {
	IsBlacklisted(ctx context.Context, entryType, targetID string) (bool, error)
	Get(ctx context.Context, entryType, targetID string) (BlacklistEntry, error)
	Add(ctx context.Context, entryType, targetID, reason string) error
	Remove(ctx context.Context, entryType, targetID string) error
	List(ctx context.Context, entryType string) ([]BlacklistEntry, error)
}

type SQLiteBlacklistRepository struct {
	read  *sql.DB
	write *sql.DB
}

func NewSQLiteBlacklistRepository(read, write *sql.DB) *SQLiteBlacklistRepository {
	return &SQLiteBlacklistRepository{read: read, write: write}
}

func (r *SQLiteBlacklistRepository) IsBlacklisted(ctx context.Context, entryType, targetID string) (bool, error) {
	var count int
	err := r.read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM blacklist_entries WHERE entry_type = ? AND target_id = ?",
		entryType, targetID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SQLiteBlacklistRepository) Add(ctx context.Context, entryType, targetID, reason string) error {
	_, err := r.write.ExecContext(ctx,
		`INSERT INTO blacklist_entries (entry_type, target_id, reason, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (entry_type, target_id) DO UPDATE SET reason = excluded.reason`,
		entryType, targetID, reason, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (r *SQLiteBlacklistRepository) Get(ctx context.Context, entryType, targetID string) (BlacklistEntry, error) {
	var entry BlacklistEntry
	err := r.read.QueryRowContext(ctx,
		`SELECT id, entry_type, target_id, reason, created_at
		 FROM blacklist_entries
		 WHERE entry_type = ? AND target_id = ?`,
		entryType, targetID,
	).Scan(&entry.ID, &entry.EntryType, &entry.TargetID, &entry.Reason, &entry.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return BlacklistEntry{}, ErrGovernanceEntryNotFound
	}
	if err != nil {
		return BlacklistEntry{}, err
	}
	return entry, nil
}

func (r *SQLiteBlacklistRepository) Remove(ctx context.Context, entryType, targetID string) error {
	result, err := r.write.ExecContext(ctx,
		"DELETE FROM blacklist_entries WHERE entry_type = ? AND target_id = ?",
		entryType, targetID)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrGovernanceEntryNotFound
	}
	return nil
}

func (r *SQLiteBlacklistRepository) List(ctx context.Context, entryType string) ([]BlacklistEntry, error) {
	rows, err := r.read.QueryContext(ctx,
		"SELECT id, entry_type, target_id, reason, created_at FROM blacklist_entries WHERE entry_type = ? ORDER BY created_at DESC",
		entryType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []BlacklistEntry
	for rows.Next() {
		var entry BlacklistEntry
		if err := rows.Scan(&entry.ID, &entry.EntryType, &entry.TargetID, &entry.Reason, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

type WhitelistEntry struct {
	ID        int64
	EntryType string
	TargetID  string
	Reason    string
	CreatedAt string
}

type WhitelistRepository interface {
	IsWhitelisted(ctx context.Context, entryType, targetID string) (bool, error)
	Get(ctx context.Context, entryType, targetID string) (WhitelistEntry, error)
	Add(ctx context.Context, entryType, targetID, reason string) error
	Remove(ctx context.Context, entryType, targetID string) error
	List(ctx context.Context, entryType string) ([]WhitelistEntry, error)
}

type WhitelistStateRepository interface {
	Enabled(ctx context.Context) (bool, error)
	SetEnabled(ctx context.Context, enabled bool) error
}

type SQLiteWhitelistRepository struct {
	read  *sql.DB
	write *sql.DB
}

func NewSQLiteWhitelistRepository(read, write *sql.DB) *SQLiteWhitelistRepository {
	return &SQLiteWhitelistRepository{read: read, write: write}
}

func (r *SQLiteWhitelistRepository) IsWhitelisted(ctx context.Context, entryType, targetID string) (bool, error) {
	var count int
	err := r.read.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM whitelist_entries WHERE entry_type = ? AND target_id = ?",
		entryType, targetID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SQLiteWhitelistRepository) Get(ctx context.Context, entryType, targetID string) (WhitelistEntry, error) {
	var entry WhitelistEntry
	err := r.read.QueryRowContext(ctx,
		`SELECT id, entry_type, target_id, reason, created_at
		 FROM whitelist_entries
		 WHERE entry_type = ? AND target_id = ?`,
		entryType, targetID,
	).Scan(&entry.ID, &entry.EntryType, &entry.TargetID, &entry.Reason, &entry.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return WhitelistEntry{}, ErrGovernanceEntryNotFound
	}
	if err != nil {
		return WhitelistEntry{}, err
	}
	return entry, nil
}

func (r *SQLiteWhitelistRepository) Add(ctx context.Context, entryType, targetID, reason string) error {
	_, err := r.write.ExecContext(ctx,
		`INSERT INTO whitelist_entries (entry_type, target_id, reason, created_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (entry_type, target_id) DO UPDATE SET reason = excluded.reason`,
		entryType, targetID, reason, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (r *SQLiteWhitelistRepository) Remove(ctx context.Context, entryType, targetID string) error {
	result, err := r.write.ExecContext(ctx,
		"DELETE FROM whitelist_entries WHERE entry_type = ? AND target_id = ?",
		entryType, targetID,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrGovernanceEntryNotFound
	}
	return nil
}

func (r *SQLiteWhitelistRepository) List(ctx context.Context, entryType string) ([]WhitelistEntry, error) {
	rows, err := r.read.QueryContext(ctx,
		"SELECT id, entry_type, target_id, reason, created_at FROM whitelist_entries WHERE entry_type = ? ORDER BY created_at DESC",
		entryType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []WhitelistEntry
	for rows.Next() {
		var entry WhitelistEntry
		if err := rows.Scan(&entry.ID, &entry.EntryType, &entry.TargetID, &entry.Reason, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

type SQLiteWhitelistStateRepository struct {
	read  *sql.DB
	write *sql.DB
}

func NewSQLiteWhitelistStateRepository(read, write *sql.DB) *SQLiteWhitelistStateRepository {
	return &SQLiteWhitelistStateRepository{read: read, write: write}
}

func (r *SQLiteWhitelistStateRepository) Enabled(ctx context.Context) (bool, error) {
	var enabled int
	err := r.read.QueryRowContext(ctx,
		"SELECT enabled FROM whitelist_state WHERE singleton_id = 1",
	).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return enabled == 1, nil
}

func (r *SQLiteWhitelistStateRepository) SetEnabled(ctx context.Context, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	_, err := r.write.ExecContext(ctx,
		`INSERT INTO whitelist_state (singleton_id, enabled, updated_at) VALUES (1, ?, ?)
		 ON CONFLICT (singleton_id) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`,
		value, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

type RateLimit struct {
	Count  int
	Window time.Duration
}

type CooldownTracker struct {
	userLimit  RateLimit
	groupLimit RateLimit
	mu         sync.Mutex
	windows    map[string]*slidingWindow
}

type slidingWindow struct {
	timestamps []time.Time
	limit      RateLimit
}

func NewCooldownTracker(userLimit, groupLimit RateLimit) *CooldownTracker {
	return &CooldownTracker{
		userLimit:  userLimit,
		groupLimit: groupLimit,
		windows:    make(map[string]*slidingWindow),
	}
}

func ParseRateLimit(raw string) (RateLimit, error) {
	raw = strings.TrimSpace(raw)
	countText, windowText, ok := strings.Cut(raw, "/")
	if !ok {
		return RateLimit{}, fmt.Errorf("invalid rate limit format %q", raw)
	}

	count, err := strconv.Atoi(strings.TrimSpace(countText))
	if err != nil || count <= 0 {
		return RateLimit{}, fmt.Errorf("invalid rate limit count %q", countText)
	}

	window, err := time.ParseDuration(strings.TrimSpace(windowText))
	if err != nil || window <= 0 {
		return RateLimit{}, fmt.Errorf("invalid rate limit window %q", windowText)
	}

	return RateLimit{
		Count:  count,
		Window: window,
	}, nil
}

func (t *CooldownTracker) Allow(key string) bool {
	if t == nil {
		return true
	}

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

func (t *CooldownTracker) limitForKey(key string) RateLimit {
	if strings.HasPrefix(key, "group:") {
		return t.groupLimit
	}
	return t.userLimit
}

func (t *CooldownTracker) Cleanup() {
	if t == nil {
		return
	}

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
