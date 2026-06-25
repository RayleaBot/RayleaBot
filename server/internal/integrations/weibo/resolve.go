package weibo

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

var weiboNumericIDPattern = regexp.MustCompile(`^[0-9]+$`)
var weiboHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)
var weiboSearchAnchorPattern = regexp.MustCompile(`(?is)<a\b[^>]+href=["'][^"']*(?:weibo\.com/(?:u/)?|m\.weibo\.cn/u/)([0-9]+)[^"']*["'][^>]*>.*?</a>`)
var weiboSearchNickPattern = regexp.MustCompile(`(?is)\bnick-name=["']([^"']+)["']`)
var weiboSearchTitlePattern = regexp.MustCompile(`(?is)\btitle=["']([^"']+)["']`)
var weiboSearchAltPattern = regexp.MustCompile(`(?is)\balt=["']([^"']+)["']`)
var weiboSearchImagePattern = regexp.MustCompile(`(?is)(?:https?:)?//[^"'\s<>]*sinaimg\.cn[^"'\s<>]+`)

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
	if len(attempts) == 0 {
		attempts = append(attempts, map[string]string{})
	}
	return attempts
}

func searchWeiboUsers(ctx context.Context, client *http.Client, cookies map[string]string, query string) ([]thirdparty.AccountProfile, error) {
	var firstErr error
	for _, searchType := range []string{"3", "1"} {
		values := url.Values{
			"containerid": {"100103type=" + searchType + "&q=" + strings.TrimSpace(query)},
			"page_type":   {"searchall"},
		}
		var response struct {
			Data map[string]any `json:"data"`
		}
		if err := getWeiboJSON(ctx, client, "https://m.weibo.cn/api/container/getIndex?"+values.Encode(), weiboProfileHeaders("https://m.weibo.cn/"), cookies, &response); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		profiles := make([]thirdparty.AccountProfile, 0, 8)
		seen := map[string]bool{}
		collectWeiboProfiles(response.Data, seen, &profiles, 0)
		if len(profiles) > 0 {
			return enrichWeiboResolveProfiles(ctx, client, cookies, profiles), nil
		}
	}
	profiles, err := searchWeiboWebUsers(ctx, client, cookies, query)
	if err != nil {
		if firstErr == nil {
			firstErr = err
		}
	} else if len(profiles) > 0 {
		return enrichWeiboResolveProfiles(ctx, client, cookies, profiles), nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, nil
}

func collectWeiboProfiles(value any, seen map[string]bool, profiles *[]thirdparty.AccountProfile, depth int) {
	if depth > maxWeiboResolveDepth || len(*profiles) >= maxWeiboResolveCandidates {
		return
	}
	switch item := value.(type) {
	case map[string]any:
		profile := weiboProfileFromSearchObject(item)
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

func enrichWeiboResolveProfiles(ctx context.Context, client *http.Client, cookies map[string]string, profiles []thirdparty.AccountProfile) []thirdparty.AccountProfile {
	for index := range profiles {
		uid := strings.TrimSpace(profiles[index].UID)
		if strings.TrimSpace(profiles[index].AvatarURL) != "" {
			profiles[index].AvatarURL = normalizeWeiboImageURL(profiles[index].AvatarURL)
			continue
		}
		if uid == "" {
			continue
		}
		attemptCookies := common.CloneStringMap(cookies)
		if detail, err := fetchWeiboMobileDetailProfile(ctx, client, attemptCookies, uid); err == nil {
			profiles[index] = common.MergeAccountProfiles(profiles[index], detail)
		}
		if strings.TrimSpace(profiles[index].AvatarURL) == "" {
			profiles[index].AvatarURL = fetchWeiboAvatarFromMobilePage(ctx, client, uid, attemptCookies)
		}
		profiles[index].AvatarURL = normalizeWeiboImageURL(profiles[index].AvatarURL)
	}
	return profiles
}

func weiboProfileFromSearchObject(object map[string]any) thirdparty.AccountProfile {
	for _, key := range []string{"user", "userInfo", "profile"} {
		if nested, ok := object[key].(map[string]any); ok {
			if profile := weiboProfileFromObject(nested); profileIsUsable(profile) {
				return profile
			}
		}
	}
	uid := common.FirstNonEmpty(
		weiboUIDFromInput(common.JSONStringValue(object["scheme"])),
		weiboUIDFromInput(common.JSONStringValue(object["profile_url"])),
		weiboUIDFromInput(common.JSONStringValue(object["url"])),
	)
	if uid == "" && weiboSearchObjectHasUserFields(object) {
		uid = common.FirstNonEmpty(
			common.JSONStringValue(object["uid"]),
			common.JSONStringValue(object["id"]),
			common.JSONStringValue(object["idstr"]),
		)
	}
	if uid == "" {
		return thirdparty.AccountProfile{}
	}
	profile := thirdparty.AccountProfile{UID: uid}
	profile.Nickname = cleanWeiboSearchText(common.FirstNonEmpty(
		common.JSONStringValue(object["screen_name"]),
		common.JSONStringValue(object["nickname"]),
		common.JSONStringValue(object["title_sub"]),
		common.JSONStringValue(object["desc1"]),
		common.JSONStringValue(object["desc"]),
	))
	if strings.TrimSpace(profile.Nickname) == "" {
		return thirdparty.AccountProfile{}
	}
	profile.UID = uid
	profile.AvatarURL = common.FirstNonEmpty(
		common.JSONStringValue(object["avatar_hd"]),
		common.JSONStringValue(object["avatar_large"]),
		common.JSONStringValue(object["profile_image_url"]),
		common.JSONStringValue(object["avatar"]),
		common.JSONStringValue(object["avatar_url"]),
		common.JSONStringValue(object["pic"]),
		common.JSONStringValue(object["image"]),
	)
	profile.AvatarURL = normalizeWeiboImageURL(profile.AvatarURL)
	return profile
}

func weiboSearchObjectHasUserFields(object map[string]any) bool {
	for _, key := range []string{"screen_name", "nickname", "avatar_hd", "avatar_large", "profile_image_url"} {
		if strings.TrimSpace(common.JSONStringValue(object[key])) != "" {
			return true
		}
	}
	return false
}

func cleanWeiboSearchText(value string) string {
	text := html.UnescapeString(strings.TrimSpace(value))
	for range 2 {
		decoded, err := url.QueryUnescape(text)
		if err != nil || decoded == text {
			break
		}
		text = decoded
	}
	text = weiboHTMLTagPattern.ReplaceAllString(text, " ")
	text = strings.Join(strings.Fields(text), " ")
	text = strings.TrimSpace(strings.TrimPrefix(text, "@"))
	text = strings.TrimSuffix(text, "的微博主页")
	text = strings.TrimSuffix(text, "的微博")
	if !weiboSearchNameUsable(text) {
		return ""
	}
	return text
}

func weiboSearchNameUsable(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" || len([]rune(text)) > 48 {
		return false
	}
	lower := strings.ToLower(text)
	if strings.ContainsAny(text, "<>=\"") || strings.Contains(text, "%") {
		return false
	}
	for _, marker := range []string{"click:user_name", "seqid:", "ext:mpos", "suda-data", "woo-button", "target=_blank", "class="} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func searchWeiboWebUsers(ctx context.Context, client *http.Client, cookies map[string]string, query string) ([]thirdparty.AccountProfile, error) {
	if client == nil {
		client = common.NewHTTPClientFollow(nil)
	} else {
		client = common.NewHTTPClientFollow(client.Transport)
	}
	values := url.Values{"q": {strings.TrimSpace(query)}}
	body, err := common.FetchPageBody(ctx, client, "https://s.weibo.com/user?"+values.Encode(), weiboSearchPageHeaders(), cookies)
	if err != nil {
		return nil, err
	}
	return weiboProfilesFromSearchPage(body), nil
}

func weiboProfilesFromSearchPage(body string) []thirdparty.AccountProfile {
	matches := weiboSearchAnchorPattern.FindAllStringSubmatchIndex(body, -1)
	profiles := make([]thirdparty.AccountProfile, 0, min(len(matches), maxWeiboResolveCandidates))
	seen := map[string]bool{}
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		uid := body[match[2]:match[3]]
		if seen[uid] {
			continue
		}
		anchor := body[match[0]:match[1]]
		start := max(0, match[0]-800)
		end := min(len(body), match[1]+800)
		chunk := body[start:end]
		name := weiboSearchNameFromHTML(anchor)
		if strings.TrimSpace(name) == "" {
			continue
		}
		seen[uid] = true
		profiles = append(profiles, thirdparty.AccountProfile{
			UID:       uid,
			Nickname:  name,
			AvatarURL: weiboSearchAvatarFromHTML(chunk),
		})
		if len(profiles) >= maxWeiboResolveCandidates {
			break
		}
	}
	return profiles
}

func weiboSearchNameFromHTML(value string) string {
	for _, pattern := range []*regexp.Regexp{weiboSearchNickPattern, weiboSearchTitlePattern, weiboSearchAltPattern} {
		if match := pattern.FindStringSubmatch(value); len(match) > 1 {
			if name := cleanWeiboSearchText(match[1]); name != "" {
				return name
			}
		}
	}
	start := strings.Index(value, ">")
	end := strings.LastIndex(strings.ToLower(value), "</a>")
	if start >= 0 && end > start {
		return cleanWeiboSearchText(value[start+1 : end])
	}
	return ""
}

func weiboSearchAvatarFromHTML(value string) string {
	if match := weiboSearchImagePattern.FindString(value); match != "" {
		return normalizeWeiboImageURL(match)
	}
	return ""
}

func normalizeWeiboImageURL(value string) string {
	text := html.UnescapeString(strings.TrimSpace(value))
	text = strings.ReplaceAll(text, `\/`, `/`)
	if strings.HasPrefix(text, "//") {
		return "https:" + text
	}
	return text
}

func weiboSearchPageHeaders() map[string]string {
	return map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Referer":         "https://s.weibo.com/",
		"User-Agent":      weiboUserAgent,
		"Cache-Control":   "no-cache",
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
	if !common.HostMatches(parsed.Hostname(), "weibo.com", "weibo.cn") {
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
