package adapter

import (
	"strconv"
	"strings"
)

func (s *Shell) invalidateIdentityCacheForEvent(event NormalizedEvent) {
	cache := s.currentIdentityCache()
	if cache == nil {
		return
	}

	groupID := strings.TrimSpace(event.ConversationID)
	userID := strings.TrimSpace(event.SenderID)

	switch strings.TrimSpace(event.EventType) {
	case "notice.group_card", "notice.group_title":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "notice.group_admin", "notice.member_decrease":
		if groupID != "" {
			cache.InvalidateGroupMembers(groupID)
		}
	case "notice.member_increase":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "notice.group_name", "notice.group_profile":
		if groupID != "" {
			cache.InvalidateGroupInfo(groupID)
		}
	}

	onebot := cloneOptionalMap(event.PayloadFields["onebot"])
	noticeType := strings.TrimSpace(payloadStringValue(onebot["notice_type"]))
	if noticeType == "" {
		noticeType = strings.TrimSpace(payloadStringValue(event.PayloadFields["notice_type"]))
	}
	subType := strings.TrimSpace(payloadStringValue(onebot["sub_type"]))
	if subType == "" {
		subType = strings.TrimSpace(payloadStringValue(event.PayloadFields["sub_type"]))
	}
	switch noticeType {
	case "group_name", "group_name_change", "group_profile":
		if groupID != "" {
			cache.InvalidateGroupInfo(groupID)
		}
	case "notify":
		switch subType {
		case "group_name", "group_name_change", "group_profile":
			if groupID != "" {
				cache.InvalidateGroupInfo(groupID)
			}
		}
	case "group_card", "group_title":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	}
}

func (s *Shell) invalidateIdentityCacheForFrame(frame oneBotFrame) {
	if strings.TrimSpace(frame.PostType) != "notice" {
		return
	}

	cache := s.currentIdentityCache()
	if cache == nil {
		return
	}

	groupID := positiveIDString(frame.GroupID)
	userID := positiveIDString(frame.UserID)

	switch strings.TrimSpace(frame.NoticeType) {
	case "group_name", "group_name_change", "group_profile":
		if groupID != "" {
			cache.InvalidateGroupInfo(groupID)
		}
	case "notify":
		switch strings.TrimSpace(frame.SubType) {
		case "group_name", "group_name_change", "group_profile":
			if groupID != "" {
				cache.InvalidateGroupInfo(groupID)
			}
		}
	case "group_card", "group_title":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "group_admin", "group_decrease":
		if groupID != "" {
			cache.InvalidateGroupMembers(groupID)
		}
	case "group_increase":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	}
}

func (s *Shell) invalidateIdentityCacheForAPICall(action string, params map[string]any) {
	cache := s.currentIdentityCache()
	if cache == nil {
		return
	}

	groupID := strings.TrimSpace(payloadStringValue(params["group_id"]))
	userID := strings.TrimSpace(payloadStringValue(params["user_id"]))

	switch strings.TrimSpace(action) {
	case "set_group_name":
		if groupID != "" {
			cache.InvalidateGroupInfo(groupID)
		}
	case "set_group_card", "set_group_special_title":
		if groupID != "" && userID != "" {
			cache.InvalidateGroupMemberInfo(groupID, userID)
		}
	case "set_group_admin":
		if groupID != "" {
			cache.InvalidateGroupMembers(groupID)
		}
	}
}

func positiveIDString(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}
