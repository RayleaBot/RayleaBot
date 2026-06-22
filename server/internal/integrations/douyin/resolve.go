package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

var douyinDataScriptPattern = regexp.MustCompile(`(?is)<script[^>]+id=["'](?:RENDER_DATA|ROUTER_DATA|__UNIVERSAL_DATA_FOR_REHYDRATION__)["'][^>]*>(.*?)</script>`)

const (
	maxDouyinResolveCandidates = 8
	maxDouyinResolveDepth      = 8
)

func ResolveUser(ctx context.Context, client *http.Client, query string) ([]thirdparty.AccountProfile, bool, error) {
	return ResolveUserWithCookies(ctx, client, query, nil)
}

func ResolveUserWithCookies(ctx context.Context, client *http.Client, query string, cookieSets []map[string]string) ([]thirdparty.AccountProfile, bool, error) {
	normalizedQuery := strings.TrimSpace(query)
	if normalizedQuery == "" {
		return nil, false, nil
	}
	var firstErr error
	for _, cookies := range douyinResolveCookieAttempts(cookieSets) {
		if secUID := douyinSecUIDFromInput(normalizedQuery); secUID != "" {
			profile, err := fetchDouyinPublicUserBySecUID(ctx, client, secUID, cookies)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
			}
			if profileIsUsable(profile) {
				return []thirdparty.AccountProfile{profile}, true, nil
			}
		}
		if profiles, err := searchDouyinUsers(ctx, client, normalizedQuery, cookies); err == nil && len(profiles) > 0 {
			return profiles, len(profiles) == 1 || exactProfileMatch(profiles, normalizedQuery), nil
		} else if err != nil && firstErr == nil {
			firstErr = err
		}
		candidates := make([]thirdparty.AccountProfile, 0, 2)
		seen := map[string]bool{}
		for _, rawURL := range douyinUserURLsFor(normalizedQuery) {
			profile, err := fetchDouyinPublicUser(ctx, client, rawURL, cookies)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if profileIsUsable(profile) {
				key := strings.TrimSpace(profile.UID)
				if !seen[key] {
					seen[key] = true
					candidates = append(candidates, profile)
				}
			}
		}
		if len(candidates) > 0 {
			return candidates, len(candidates) == 1 || exactProfileMatch(candidates, normalizedQuery), nil
		}
	}
	if firstErr != nil {
		return nil, false, firstErr
	}
	return nil, false, nil
}

func douyinResolveCookieAttempts(cookieSets []map[string]string) []map[string]string {
	attempts := make([]map[string]string, 0, len(cookieSets)+1)
	for _, cookies := range cookieSets {
		if len(cookies) > 0 {
			attempts = append(attempts, common.CloneStringMap(cookies))
		}
	}
	attempts = append(attempts, map[string]string{})
	return attempts
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
	host := strings.ToLower(parsed.Hostname())
	if !strings.HasSuffix(host, "douyin.com") && !strings.HasSuffix(host, "iesdouyin.com") && !strings.HasSuffix(host, "amemv.com") {
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

func fetchDouyinPublicUserBySecUID(ctx context.Context, client *http.Client, secUID string, cookies map[string]string) (thirdparty.AccountProfile, error) {
	values := douyinWebParams()
	values.Set("sec_user_id", strings.TrimSpace(secUID))
	rawURL := "https://www.douyin.com/aweme/v1/web/user/profile/other/?" + values.Encode()
	document, err := getDouyinJSON(ctx, client, rawURL, douyinHeaders(), cookies)
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return douyinProfileFromValue(document), nil
}

func searchDouyinUsers(ctx context.Context, client *http.Client, query string, cookies map[string]string) ([]thirdparty.AccountProfile, error) {
	values := douyinWebParams()
	values.Set("keyword", strings.TrimSpace(query))
	values.Set("search_channel", "aweme_user_web")
	values.Set("search_source", "normal_search")
	values.Set("type", "user")
	values.Set("offset", "0")
	values.Set("count", strconv.Itoa(maxDouyinResolveCandidates))
	rawURL := "https://www.douyin.com/aweme/v1/web/general/search/single/?" + values.Encode()
	document, err := getDouyinJSON(ctx, client, rawURL, douyinHeaders(), cookies)
	if err != nil {
		return nil, err
	}
	profiles := make([]thirdparty.AccountProfile, 0, maxDouyinResolveCandidates)
	seen := map[string]bool{}
	collectDouyinProfiles(document, seen, &profiles, 0)
	return profiles, nil
}

func douyinWebParams() url.Values {
	return url.Values{
		"device_platform": {"webapp"},
		"aid":             {"6383"},
		"channel":         {"channel_pc_web"},
		"pc_client_type":  {"1"},
	}
}

func getDouyinJSON(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, cookies map[string]string) (any, error) {
	if client == nil {
		client = common.NewHTTPClientFollow(nil)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	common.ApplyHeaders(request, headers, cookies)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	common.MergeResponseCookies(cookies, response)
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("douyin resolve http %d", response.StatusCode)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return map[string]any{}, nil
	}
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	if object, ok := document.(map[string]any); ok {
		statusCode := common.JSONStringValue(object["status_code"])
		if statusCode != "" && statusCode != "0" {
			return map[string]any{}, nil
		}
	}
	return document, nil
}

func douyinUserURLsFor(query string) []string {
	text := strings.TrimSpace(query)
	if parsed, err := url.Parse(text); err == nil && parsed.Host != "" {
		host := strings.ToLower(parsed.Hostname())
		if strings.HasSuffix(host, "douyin.com") || strings.HasSuffix(host, "iesdouyin.com") || strings.HasSuffix(host, "amemv.com") {
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

func fetchDouyinPublicUser(ctx context.Context, client *http.Client, rawURL string, cookies map[string]string) (thirdparty.AccountProfile, error) {
	if client == nil {
		client = common.NewHTTPClientFollow(nil)
	} else {
		client = common.NewHTTPClientFollow(client.Transport)
	}
	body, err := common.FetchPageBody(ctx, client, rawURL, douyinHeaders(), cookies)
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return douyinProfileFromPage(body), nil
}

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
