package onebot11

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// identityCacheMaxEntries bounds each identity map. Entries expire on read,
// so the bound only matters for identities that are never looked up again.
const identityCacheMaxEntries = 4096

// IdentityCache provides TTL-based caching for OneBot11 identity lookups
// (login info, group info, group member info, stranger info). Expired
// entries are detected on read; a write that pushes a map past
// identityCacheMaxEntries evicts expired entries first, then the ones
// closest to expiry.
type IdentityCache struct {
	ttl            time.Duration
	mu             sync.RWMutex
	login          *cachedLogin
	groups         map[string]*cachedIdentity[GroupInfo]
	members        map[string]*cachedIdentity[GroupMemberInfo]
	strangers      map[string]*cachedIdentity[StrangerInfo]
	groupExpiry    identityExpiryQueue
	memberExpiry   identityExpiryQueue
	strangerExpiry identityExpiryQueue
}

type cachedLogin struct {
	value     LoginInfo
	expiresAt time.Time
}

// NewIdentityCache creates a new cache with the given entry TTL.
func NewIdentityCache(ttl time.Duration) *IdentityCache {
	return &IdentityCache{
		ttl:       ttl,
		groups:    make(map[string]*cachedIdentity[GroupInfo]),
		members:   make(map[string]*cachedIdentity[GroupMemberInfo]),
		strangers: make(map[string]*cachedIdentity[StrangerInfo]),
	}
}

// Clear invalidates all cached entries.
func (c *IdentityCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.login = nil
	c.groups = make(map[string]*cachedIdentity[GroupInfo])
	c.members = make(map[string]*cachedIdentity[GroupMemberInfo])
	c.strangers = make(map[string]*cachedIdentity[StrangerInfo])
	c.groupExpiry, c.memberExpiry, c.strangerExpiry = nil, nil, nil
}

// GetStrangerInfo returns the cached stranger info if present and not expired.
func (c *IdentityCache) GetStrangerInfo(userID string) (StrangerInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry := c.strangers[userID]
	if entry == nil || time.Now().After(entry.expiresAt) {
		return StrangerInfo{}, false
	}
	return entry.value, true
}

// SetStrangerInfo stores stranger info in the cache.
func (c *IdentityCache) SetStrangerInfo(userID string, info StrangerInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	setIdentityEntry(c.strangers, &c.strangerExpiry, userID, info, time.Now(), c.ttl)
}

// GetLogin returns the cached login info if present and not expired.
func (c *IdentityCache) GetLogin() (LoginInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.login == nil || time.Now().After(c.login.expiresAt) {
		return LoginInfo{}, false
	}
	return c.login.value, true
}

// SetLogin stores login info in the cache.
func (c *IdentityCache) SetLogin(info LoginInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.login = &cachedLogin{
		value:     info,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// GetGroupInfo returns the cached group info if present and not expired.
func (c *IdentityCache) GetGroupInfo(groupID string) (GroupInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry := c.groups[groupID]
	if entry == nil || time.Now().After(entry.expiresAt) {
		return GroupInfo{}, false
	}
	return entry.value, true
}

// SetGroupInfo stores group info in the cache.
func (c *IdentityCache) SetGroupInfo(groupID string, info GroupInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	setIdentityEntry(c.groups, &c.groupExpiry, groupID, info, time.Now(), c.ttl)
}

func (c *IdentityCache) InvalidateGroupInfo(groupID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	deleteIdentityEntry(c.groups, &c.groupExpiry, groupID)
}

// GetGroupMemberInfo returns the cached group member info if present and
// not expired. The cache key combines groupID and userID.
func (c *IdentityCache) GetGroupMemberInfo(groupID, userID string) (GroupMemberInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := groupID + ":" + userID
	entry := c.members[key]
	if entry == nil || time.Now().After(entry.expiresAt) {
		return GroupMemberInfo{}, false
	}
	return entry.value, true
}

// SetGroupMemberInfo stores group member info in the cache.
func (c *IdentityCache) SetGroupMemberInfo(groupID, userID string, info GroupMemberInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	setIdentityEntry(c.members, &c.memberExpiry, groupID+":"+userID, info, time.Now(), c.ttl)
}

func (c *IdentityCache) InvalidateGroupMemberInfo(groupID, userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	deleteIdentityEntry(c.members, &c.memberExpiry, groupID+":"+userID)
}

func (c *IdentityCache) InvalidateGroupMembers(groupID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := groupID + ":"
	for key := range c.members {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			deleteIdentityEntry(c.members, &c.memberExpiry, key)
		}
	}
}

type EventInvalidation struct {
	EventType      string
	ConversationID string
	SenderID       string
	PayloadFields  map[string]any
}

type FrameInvalidation struct {
	PostType   string
	NoticeType string
	SubType    string
	GroupID    int64
	UserID     int64
}

func (c *IdentityCache) InvalidateForEvent(event EventInvalidation) {
	groupID := strings.TrimSpace(event.ConversationID)
	userID := strings.TrimSpace(event.SenderID)

	switch strings.TrimSpace(event.EventType) {
	case "notice.group_card", "notice.group_title":
		if groupID != "" && userID != "" {
			c.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "notice.group_admin", "notice.member_decrease":
		if groupID != "" {
			c.InvalidateGroupMembers(groupID)
		}
	case "notice.member_increase":
		if groupID != "" && userID != "" {
			c.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "notice.group_name", "notice.group_profile":
		if groupID != "" {
			c.InvalidateGroupInfo(groupID)
		}
	}

	onebot := cachePayloadMap(event.PayloadFields["onebot"])
	noticeType := strings.TrimSpace(cachePayloadString(onebot["notice_type"]))
	if noticeType == "" {
		noticeType = strings.TrimSpace(cachePayloadString(event.PayloadFields["notice_type"]))
	}
	subType := strings.TrimSpace(cachePayloadString(onebot["sub_type"]))
	if subType == "" {
		subType = strings.TrimSpace(cachePayloadString(event.PayloadFields["sub_type"]))
	}
	switch noticeType {
	case "group_name", "group_name_change", "group_profile":
		if groupID != "" {
			c.InvalidateGroupInfo(groupID)
		}
	case "notify":
		switch subType {
		case "group_name", "group_name_change", "group_profile":
			if groupID != "" {
				c.InvalidateGroupInfo(groupID)
			}
		}
	case "group_card", "group_title":
		if groupID != "" && userID != "" {
			c.InvalidateGroupMemberInfo(groupID, userID)
		}
	}
}

func (c *IdentityCache) InvalidateForFrame(frame FrameInvalidation) {
	if strings.TrimSpace(frame.PostType) != "notice" {
		return
	}

	groupID := positiveIDString(frame.GroupID)
	userID := positiveIDString(frame.UserID)

	switch strings.TrimSpace(frame.NoticeType) {
	case "group_name", "group_name_change", "group_profile":
		if groupID != "" {
			c.InvalidateGroupInfo(groupID)
		}
	case "notify":
		switch strings.TrimSpace(frame.SubType) {
		case "group_name", "group_name_change", "group_profile":
			if groupID != "" {
				c.InvalidateGroupInfo(groupID)
			}
		}
	case "group_card", "group_title":
		if groupID != "" && userID != "" {
			c.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "group_admin", "group_decrease":
		if groupID != "" {
			c.InvalidateGroupMembers(groupID)
		}
	case "group_increase":
		if groupID != "" && userID != "" {
			c.InvalidateGroupMemberInfo(groupID, userID)
		}
	}
}

func (c *IdentityCache) InvalidateForAPICall(action string, params map[string]any) {
	groupID := strings.TrimSpace(cachePayloadString(params["group_id"]))
	userID := strings.TrimSpace(cachePayloadString(params["user_id"]))

	switch strings.TrimSpace(action) {
	case "set_group_name":
		if groupID != "" {
			c.InvalidateGroupInfo(groupID)
		}
	case "set_group_card", "set_group_special_title":
		if groupID != "" && userID != "" {
			c.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "set_group_admin":
		if groupID != "" {
			c.InvalidateGroupMembers(groupID)
		}
	}
}

func positiveIDString(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func cachePayloadMap(value any) map[string]any {
	typed, _ := value.(map[string]any)
	return typed
}

func cachePayloadString(value any) string {
	if value == nil {
		return ""
	}
	valueString := strings.TrimSpace(fmt.Sprint(value))
	if valueString == "<nil>" {
		return ""
	}
	return valueString
}
