package onebot11

import (
	"fmt"
	"strconv"
	"strings"
)

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
