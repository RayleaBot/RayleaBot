package douyin

import (
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

var douyinDataScriptPattern = regexp.MustCompile(`(?is)<script[^>]+id=["'](?:RENDER_DATA|ROUTER_DATA|__UNIVERSAL_DATA_FOR_REHYDRATION__)["'][^>]*>(.*?)</script>`)

func douyinProfileFromPage(body string) thirdparty.AccountProfile {
	for _, match := range douyinDataScriptPattern.FindAllStringSubmatch(body, -1) {
		if len(match) < 2 {
			continue
		}
		decoded := html.UnescapeString(strings.TrimSpace(match[1]))
		if unescaped, err := url.QueryUnescape(decoded); err == nil {
			decoded = unescaped
		}
		var document any
		if err := json.Unmarshal([]byte(decoded), &document); err != nil {
			continue
		}
		if profile := douyinProfileFromValue(document); profileIsUsable(profile) {
			return profile
		}
	}
	return thirdparty.AccountProfile{}
}

func douyinProfileFromUserPayload(value any) thirdparty.AccountProfile {
	object, ok := value.(map[string]any)
	if !ok {
		return thirdparty.AccountProfile{}
	}
	for _, key := range []string{"user", "user_info"} {
		if user, ok := object[key].(map[string]any); ok {
			if profile := douyinProfileFromObject(user); profileIsUsable(profile) {
				return profile
			}
		}
	}
	return thirdparty.AccountProfile{}
}

func douyinProfileFromValue(value any) thirdparty.AccountProfile {
	return douyinProfileFromValueAtDepth(value, 0)
}

func douyinProfileFromValueAtDepth(value any, depth int) thirdparty.AccountProfile {
	if depth > maxDouyinResolveDepth {
		return thirdparty.AccountProfile{}
	}
	switch item := value.(type) {
	case map[string]any:
		profile := douyinProfileFromObject(item)
		if profileIsUsable(profile) {
			return profile
		}
		for _, child := range item {
			if nested := douyinProfileFromValueAtDepth(child, depth+1); profileIsUsable(nested) {
				return nested
			}
		}
	case []any:
		for _, child := range item {
			if nested := douyinProfileFromValueAtDepth(child, depth+1); profileIsUsable(nested) {
				return nested
			}
		}
	}
	return thirdparty.AccountProfile{}
}

func douyinProfileFromObject(object map[string]any) thirdparty.AccountProfile {
	profile := thirdparty.AccountProfile{
		UID:      common.FirstNonEmpty(common.JSONStringValue(object["unique_id"]), common.JSONStringValue(object["short_id"]), common.JSONStringValue(object["uid"]), common.JSONStringValue(object["sec_uid"])),
		Nickname: common.JSONStringValue(object["nickname"]),
	}
	profile.AvatarURL = douyinAvatarURLFromObject(object)
	return profile
}

func collectDouyinProfiles(value any, seen map[string]bool, profiles *[]thirdparty.AccountProfile, depth int) {
	if depth > maxDouyinResolveDepth || len(*profiles) >= maxDouyinResolveCandidates {
		return
	}
	switch item := value.(type) {
	case map[string]any:
		profile := douyinProfileFromObject(item)
		if profileIsUsable(profile) {
			key := strings.TrimSpace(profile.UID)
			if !seen[key] {
				seen[key] = true
				*profiles = append(*profiles, profile)
			}
		}
		for _, child := range item {
			collectDouyinProfiles(child, seen, profiles, depth+1)
			if len(*profiles) >= maxDouyinResolveCandidates {
				return
			}
		}
	case []any:
		for _, child := range item {
			collectDouyinProfiles(child, seen, profiles, depth+1)
			if len(*profiles) >= maxDouyinResolveCandidates {
				return
			}
		}
	}
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
					if text := common.JSONStringValue(item); strings.TrimSpace(text) != "" {
						return text
					}
				}
			}
			if text := common.FirstNonEmpty(common.JSONStringValue(avatar["url"]), common.JSONStringValue(avatar["uri"])); text != "" {
				return text
			}
		}
	}
	return common.FirstNonEmpty(common.JSONStringValue(object["avatar_url"]), common.JSONStringValue(object["avatar"]))
}

func profileIsUsable(profile thirdparty.AccountProfile) bool {
	return strings.TrimSpace(profile.UID) != "" && strings.TrimSpace(profile.Nickname) != ""
}

func exactProfileMatch(profiles []thirdparty.AccountProfile, query string) bool {
	normalized := strings.TrimSpace(strings.ToLower(strings.TrimPrefix(query, "@")))
	for _, profile := range profiles {
		if strings.ToLower(strings.TrimSpace(profile.UID)) == normalized || strings.ToLower(strings.TrimSpace(profile.Nickname)) == normalized {
			return true
		}
	}
	return false
}
