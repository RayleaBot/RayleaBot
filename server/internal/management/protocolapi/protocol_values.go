package protocolapi

import (
	"net/http"
	"strings"
)

func allowOneBotIngress(r *http.Request, accessToken string, allowQueryToken bool) bool {
	trimmedToken := strings.TrimSpace(accessToken)
	if trimmedToken == "" {
		return true
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		if strings.TrimSpace(authHeader[7:]) == trimmedToken {
			return true
		}
	}
	if allowQueryToken && strings.TrimSpace(r.URL.Query().Get("access_token")) == trimmedToken {
		return true
	}
	return false
}

func currentOneBotProvider(raw string) string {
	switch strings.TrimSpace(raw) {
	case "standard", "napcat", "luckylillia":
		return strings.TrimSpace(raw)
	default:
		return "unknown"
	}
}

func oneBot11AvatarURL(userID string) string {
	return "https://q1.qlogo.cn/g?b=qq&nk=" + strings.TrimSpace(userID) + "&s=640"
}

func oneBot11GroupAvatarURL(groupID string) string {
	id := strings.TrimSpace(groupID)
	if id == "" {
		return ""
	}
	return "https://p.qlogo.cn/gh/" + id + "/" + id + "/100"
}

func oneBot11RoleLabel(role string) string {
	switch strings.TrimSpace(role) {
	case "owner":
		return "群主"
	case "admin":
		return "管理员"
	case "member":
		return "成员"
	default:
		return ""
	}
}

func isDigits(raw string) bool {
	if raw == "" {
		return false
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
