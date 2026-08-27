package douyin

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

const (
	maxDouyinResolveCandidates = 8
	maxDouyinResolveDepth      = 8
)

func douyinSearchProfilesFromDocument(document any, query string) []thirdparty.AccountProfile {
	profiles := make([]thirdparty.AccountProfile, 0, maxDouyinResolveCandidates)
	seen := map[string]bool{}
	collectDouyinSearchProfiles(document, seen, &profiles, 0, false)
	return filterDouyinProfilesForQuery(profiles, query)
}

func douyinSearchProfilesFromJSON(body string, query string) ([]thirdparty.AccountProfile, error) {
	text := strings.TrimSpace(body)
	if text == "" {
		return nil, nil
	}
	var check struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(text), &check); err == nil && strings.TrimSpace(check.Error) != "" {
		return nil, fmt.Errorf("douyin browser search: %s", check.Error)
	}
	var document any
	if err := json.Unmarshal([]byte(text), &document); err != nil {
		return nil, err
	}
	if object, ok := document.(map[string]any); ok {
		statusCode := thirdparty.JSONStringValue(object["status_code"])
		if statusCode != "" && statusCode != "0" {
			return nil, nil
		}
	}
	return douyinSearchProfilesFromDocument(document, query), nil
}

func filterDouyinProfilesForQuery(profiles []thirdparty.AccountProfile, query string) []thirdparty.AccountProfile {
	normalized := normalizedDouyinQuery(query)
	if normalized == "" {
		return profiles
	}
	filtered := make([]thirdparty.AccountProfile, 0, len(profiles))
	for _, profile := range profiles {
		if douyinProfileMatchesQuery(profile, normalized) {
			filtered = append(filtered, profile)
		}
	}
	return filtered
}

func normalizedDouyinQuery(query string) string {
	text := strings.TrimSpace(strings.TrimPrefix(query, "@"))
	if parsed, err := url.Parse(text); err == nil && parsed.Host != "" {
		parts := strings.FieldsFunc(strings.Trim(parsed.Path, "/"), func(r rune) bool {
			return r == '/'
		})
		if len(parts) > 0 {
			text = parts[len(parts)-1]
		}
	}
	return strings.ToLower(strings.TrimSpace(text))
}

func douyinProfileMatchesQuery(profile thirdparty.AccountProfile, query string) bool {
	// unique_id（抖音号）是用户搜索的常见输入，必须参与匹配；
	// UID 为稳定 sec_uid，绑定与展示均需要命中。
	for _, value := range []string{profile.UID, profile.UniqueID, profile.Nickname} {
		normalized := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "@")))
		if normalized == "" {
			continue
		}
		if normalized == query || strings.Contains(normalized, query) || strings.Contains(query, normalized) {
			return true
		}
	}
	return false
}

func douyinProfileFromObject(object map[string]any) thirdparty.AccountProfile {
	// UID 必须是稳定的 sec_uid：抖音号（unique_id）可被用户修改，不能作为
	// 订阅绑定标识；unique_id 只用于展示。
	profile := thirdparty.AccountProfile{
		UID:      thirdparty.FirstNonEmpty(thirdparty.JSONStringValue(object["sec_uid"]), thirdparty.JSONStringValue(object["uid"])),
		UniqueID: thirdparty.FirstNonEmpty(thirdparty.JSONStringValue(object["unique_id"]), thirdparty.JSONStringValue(object["short_id"])),
		Nickname: thirdparty.JSONStringValue(object["nickname"]),
	}
	profile.AvatarURL = douyinAvatarURLFromObject(object)
	return profile
}

func collectDouyinSearchProfiles(value any, seen map[string]bool, profiles *[]thirdparty.AccountProfile, depth int, inSearchResult bool) {
	if depth > maxDouyinResolveDepth || len(*profiles) >= maxDouyinResolveCandidates {
		return
	}
	switch item := value.(type) {
	case map[string]any:
		if data, ok := item["data"]; ok {
			collectDouyinSearchProfiles(data, seen, profiles, depth+1, true)
		}
		if userList, ok := item["user_list"].([]any); ok {
			for _, child := range userList {
				collectDouyinSearchProfiles(child, seen, profiles, depth+1, true)
				if len(*profiles) >= maxDouyinResolveCandidates {
					return
				}
			}
		}
		if inSearchResult {
			for _, key := range []string{"user_info", "user", "author", "author_user_info"} {
				if userInfo, ok := item[key].(map[string]any); ok {
					addDouyinProfile(userInfo, seen, profiles)
				}
			}
		}
		for _, child := range item {
			collectDouyinSearchProfiles(child, seen, profiles, depth+1, inSearchResult)
			if len(*profiles) >= maxDouyinResolveCandidates {
				return
			}
		}
	case []any:
		for _, child := range item {
			collectDouyinSearchProfiles(child, seen, profiles, depth+1, inSearchResult)
			if len(*profiles) >= maxDouyinResolveCandidates {
				return
			}
		}
	}
}

func addDouyinProfile(object map[string]any, seen map[string]bool, profiles *[]thirdparty.AccountProfile) {
	profile := douyinProfileFromObject(object)
	if !profileIsUsable(profile) {
		return
	}
	key := strings.TrimSpace(profile.UID)
	if seen[key] {
		return
	}
	seen[key] = true
	*profiles = append(*profiles, profile)
}

func douyinAvatarURLFromObject(object map[string]any) string {
	for _, key := range []string{"avatar_medium", "avatar_thumb", "avatar_larger"} {
		if avatar, ok := object[key].(map[string]any); ok {
			if urlList, ok := avatar["url_list"].([]any); ok {
				for _, item := range urlList {
					if text := thirdparty.JSONStringValue(item); strings.TrimSpace(text) != "" {
						return text
					}
				}
			}
			if text := thirdparty.FirstNonEmpty(thirdparty.JSONStringValue(avatar["url"]), thirdparty.JSONStringValue(avatar["uri"])); text != "" {
				return text
			}
		}
	}
	return thirdparty.FirstNonEmpty(thirdparty.JSONStringValue(object["avatar_url"]), thirdparty.JSONStringValue(object["avatar"]))
}

func profileIsUsable(profile thirdparty.AccountProfile) bool {
	return strings.TrimSpace(profile.UID) != "" && strings.TrimSpace(profile.Nickname) != ""
}

func exactProfileMatch(profiles []thirdparty.AccountProfile, query string) bool {
	normalized := strings.TrimSpace(strings.ToLower(strings.TrimPrefix(query, "@")))
	for _, profile := range profiles {
		for _, value := range []string{profile.UID, profile.UniqueID, profile.Nickname} {
			if strings.ToLower(strings.TrimSpace(value)) == normalized {
				return true
			}
		}
	}
	return false
}
