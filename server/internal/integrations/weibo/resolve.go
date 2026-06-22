package weibo

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

var weiboNumericIDPattern = regexp.MustCompile(`^[0-9]+$`)

const (
	maxWeiboResolveCandidates = 8
	maxWeiboResolveDepth      = 8
)

func ResolveUser(ctx context.Context, client *http.Client, query string) ([]thirdparty.AccountProfile, bool, error) {
	return ResolveUserWithCookies(ctx, client, query, nil)
}

func ResolveUserWithCookies(ctx context.Context, client *http.Client, query string, cookieSets []map[string]string) ([]thirdparty.AccountProfile, bool, error) {
	normalizedQuery := strings.TrimSpace(query)
	if normalizedQuery == "" {
		return nil, false, nil
	}
	if uid := weiboUIDFromInput(normalizedQuery); uid != "" {
		var firstErr error
		for _, cookies := range weiboResolveCookieAttempts(cookieSets) {
			profile, err := fetchWeiboMobileDetailProfile(ctx, client, cookies, uid)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			profile.UID = common.FirstNonEmpty(profile.UID, uid)
			if strings.TrimSpace(profile.AvatarURL) == "" {
				profile.AvatarURL = fetchWeiboAvatarFromMobilePage(ctx, client, profile.UID, cookies)
			}
			if profileIsUsable(profile) {
				return []thirdparty.AccountProfile{profile}, true, nil
			}
		}
		if firstErr != nil {
			return nil, false, firstErr
		}
		return nil, false, nil
	}

	var firstErr error
	for _, cookies := range weiboResolveCookieAttempts(cookieSets) {
		candidates, err := searchWeiboUsers(ctx, client, cookies, normalizedQuery)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(candidates) > 0 {
			return candidates, exactProfileMatch(candidates, normalizedQuery), nil
		}
	}
	if firstErr != nil {
		return nil, false, firstErr
	}
	return nil, false, nil
}

func weiboResolveCookieAttempts(cookieSets []map[string]string) []map[string]string {
	attempts := make([]map[string]string, 0, len(cookieSets)+1)
	for _, cookies := range cookieSets {
		if len(cookies) > 0 {
			attempts = append(attempts, common.CloneStringMap(cookies))
		}
	}
	attempts = append(attempts, map[string]string{})
	return attempts
}

func searchWeiboUsers(ctx context.Context, client *http.Client, cookies map[string]string, query string) ([]thirdparty.AccountProfile, error) {
	values := url.Values{
		"containerid": {"100103type=3&q=" + strings.TrimSpace(query)},
		"page_type":   {"searchall"},
	}
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := getWeiboJSON(ctx, client, "https://m.weibo.cn/api/container/getIndex?"+values.Encode(), weiboProfileHeaders("https://m.weibo.cn/"), cookies, &response); err != nil {
		return nil, err
	}
	profiles := make([]thirdparty.AccountProfile, 0, 8)
	seen := map[string]bool{}
	collectWeiboProfiles(response.Data, seen, &profiles, 0)
	return profiles, nil
}

func collectWeiboProfiles(value any, seen map[string]bool, profiles *[]thirdparty.AccountProfile, depth int) {
	if depth > maxWeiboResolveDepth || len(*profiles) >= maxWeiboResolveCandidates {
		return
	}
	switch item := value.(type) {
	case map[string]any:
		profile := weiboProfileFromObject(item)
		if profileIsUsable(profile) {
			key := strings.TrimSpace(profile.UID)
			if !seen[key] {
				seen[key] = true
				*profiles = append(*profiles, profile)
			}
		}
		for _, child := range item {
			collectWeiboProfiles(child, seen, profiles, depth+1)
			if len(*profiles) >= maxWeiboResolveCandidates {
				return
			}
		}
	case []any:
		for _, child := range item {
			collectWeiboProfiles(child, seen, profiles, depth+1)
			if len(*profiles) >= maxWeiboResolveCandidates {
				return
			}
		}
	}
}

func weiboUIDFromInput(query string) string {
	text := strings.TrimSpace(query)
	if weiboNumericIDPattern.MatchString(text) {
		return text
	}
	parsed, err := url.Parse(text)
	if err != nil || parsed.Host == "" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if !strings.HasSuffix(host, "weibo.com") && !strings.HasSuffix(host, "weibo.cn") {
		return ""
	}
	values := parsed.Query()
	for _, key := range []string{"uid", "value"} {
		if candidate := strings.TrimSpace(values.Get(key)); weiboNumericIDPattern.MatchString(candidate) {
			return candidate
		}
	}
	parts := strings.FieldsFunc(strings.Trim(parsed.Path, "/"), func(r rune) bool {
		return r == '/'
	})
	for index, part := range parts {
		candidate := strings.TrimSpace(part)
		if candidate == "u" || candidate == "profile" {
			if len(parts) > index+1 && weiboNumericIDPattern.MatchString(parts[index+1]) {
				return parts[index+1]
			}
			continue
		}
		if weiboNumericIDPattern.MatchString(candidate) {
			return candidate
		}
	}
	return ""
}

func profileIsUsable(profile thirdparty.AccountProfile) bool {
	return strings.TrimSpace(profile.UID) != "" && strings.TrimSpace(profile.Nickname) != ""
}

func exactProfileMatch(profiles []thirdparty.AccountProfile, query string) bool {
	normalized := strings.TrimSpace(strings.ToLower(query))
	for _, profile := range profiles {
		if strings.ToLower(strings.TrimSpace(profile.UID)) == normalized || strings.ToLower(strings.TrimSpace(profile.Nickname)) == normalized {
			return true
		}
	}
	return false
}
