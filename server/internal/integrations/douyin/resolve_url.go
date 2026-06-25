package douyin

import (
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"net/url"
	"strings"
)

func douyinIsDirectProfileInput(query string) bool {
	text := strings.TrimSpace(query)
	if strings.HasPrefix(text, "MS4w") {
		return true
	}
	parsed, err := url.Parse(text)
	if err != nil || parsed.Host == "" {
		return false
	}
	return thirdparty.HostMatches(parsed.Hostname(), "douyin.com", "iesdouyin.com", "amemv.com")
}

func douyinSecUIDFromInput(query string) string {
	text := strings.TrimSpace(query)
	parsed, err := url.Parse(text)
	if err != nil || parsed.Host == "" {
		if strings.HasPrefix(text, "MS4w") {
			return text
		}
		return ""
	}
	if !thirdparty.HostMatches(parsed.Hostname(), "douyin.com", "iesdouyin.com", "amemv.com") {
		return ""
	}
	for _, key := range []string{"sec_uid", "sec_user_id"} {
		if candidate := strings.TrimSpace(parsed.Query().Get(key)); candidate != "" {
			return candidate
		}
	}
	parts := strings.FieldsFunc(strings.Trim(parsed.Path, "/"), func(r rune) bool {
		return r == '/'
	})
	for index, part := range parts {
		if part == "user" && len(parts) > index+1 && strings.TrimSpace(parts[index+1]) != "" {
			return strings.TrimSpace(parts[index+1])
		}
	}
	return ""
}

func douyinUserURLsFor(query string) []string {
	text := strings.TrimSpace(query)
	if parsed, err := url.Parse(text); err == nil && parsed.Host != "" {
		if thirdparty.HostMatches(parsed.Hostname(), "douyin.com", "iesdouyin.com", "amemv.com") {
			return []string{text}
		}
	}
	identifier := strings.Trim(strings.TrimPrefix(text, "@"), "/")
	if strings.HasPrefix(identifier, "user/") {
		identifier = strings.TrimPrefix(identifier, "user/")
	}
	escapedIdentifier := url.PathEscape(identifier)
	return []string{
		"https://www.douyin.com/user/" + escapedIdentifier,
		"https://www.douyin.com/search/" + escapedIdentifier + "?type=user",
	}
}
