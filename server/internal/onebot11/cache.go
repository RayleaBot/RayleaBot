package onebot11

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IdentityCache provides TTL-based caching for OneBot11 identity lookups
// (login info, group info, group member info, stranger info). Expired
// entries are detected on read; no background reaper is needed.
type IdentityCache struct {
	ttl       time.Duration
	mu        sync.RWMutex
	login     *cachedLogin
	groups    map[string]*cachedGroupInfo
	members   map[string]*cachedGroupMemberInfo
	strangers map[string]*cachedStrangerInfo
}

type cachedLogin struct {
	value     LoginInfo
	expiresAt time.Time
}

type cachedGroupInfo struct {
	value     GroupInfo
	expiresAt time.Time
}

type cachedGroupMemberInfo struct {
	value     GroupMemberInfo
	expiresAt time.Time
}

type cachedStrangerInfo struct {
	value     StrangerInfo
	expiresAt time.Time
}

// NewIdentityCache creates a new cache with the given entry TTL.
func NewIdentityCache(ttl time.Duration) *IdentityCache {
	return &IdentityCache{
		ttl:       ttl,
		groups:    make(map[string]*cachedGroupInfo),
		members:   make(map[string]*cachedGroupMemberInfo),
		strangers: make(map[string]*cachedStrangerInfo),
	}
}

// Clear invalidates all cached entries.
func (c *IdentityCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.login = nil
	c.groups = make(map[string]*cachedGroupInfo)
	c.members = make(map[string]*cachedGroupMemberInfo)
	c.strangers = make(map[string]*cachedStrangerInfo)
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

	c.strangers[userID] = &cachedStrangerInfo{
		value:     info,
		expiresAt: time.Now().Add(c.ttl),
	}
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

	c.groups[groupID] = &cachedGroupInfo{
		value:     info,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *IdentityCache) InvalidateGroupInfo(groupID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.groups, groupID)
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

	key := groupID + ":" + userID
	c.members[key] = &cachedGroupMemberInfo{
		value:     info,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *IdentityCache) InvalidateGroupMemberInfo(groupID, userID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.members, groupID+":"+userID)
}

func (c *IdentityCache) InvalidateGroupMembers(groupID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	prefix := groupID + ":"
	for key := range c.members {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.members, key)
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
	if c == nil {
		return
	}

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
	if c == nil || strings.TrimSpace(frame.PostType) != "notice" {
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
	if c == nil {
		return
	}

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
	if len(typed) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(typed))
	for key, item := range typed {
		cloned[key] = item
	}
	return cloned
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
