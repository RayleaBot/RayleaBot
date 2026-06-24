package weibo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

const (
	weiboMobileConfigURL = "https://m.weibo.cn/api/config"
	weiboSideConfigURL   = "https://weibo.com/ajax/side/config"
)

func FetchAccountProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	if !weiboHasLoginCookie(cookies) {
		return thirdparty.AccountProfile{}, fmt.Errorf("weibo cookie missing login state")
	}
	var profile thirdparty.AccountProfile
	if configProfile, err := fetchWeiboMobileConfigProfile(ctx, client, cookies); err == nil {
		profile = common.MergeAccountProfiles(profile, configProfile)
	}
	// Always fetch side config too — it may contain the avatar URL even when
	// the mobile config already provided UID and nickname.
	if configProfile, err := fetchWeiboSideConfigProfile(ctx, client, cookies); err == nil {
		profile = common.MergeAccountProfiles(profile, configProfile)
	}
	if strings.TrimSpace(profile.UID) != "" {
		// Visit m.weibo.cn first to obtain its domain-specific X-CSRF-TOKEN.
		// Use FollowGet to manually track every redirect hop and preserve
		// cookies set at intermediate redirects (e.g., passport.weibo.cn).
		_ = common.FollowGet(ctx, client, "https://m.weibo.cn/", weiboProfileHeaders("https://m.weibo.cn/"), cookies)
		if detailProfile, err := fetchWeiboMobileDetailProfile(ctx, client, cookies, profile.UID); err == nil {
			profile = common.MergeAccountProfiles(profile, detailProfile)
		}
		if detailProfile, err := fetchWeiboAjaxProfile(ctx, client, cookies, profile.UID); err == nil {
			profile = common.MergeAccountProfiles(profile, detailProfile)
		}
	}
	// If avatar is still empty, try fetching the mobile user page to extract
	// the avatar from Open Graph meta tags.
	if strings.TrimSpace(profile.AvatarURL) == "" && strings.TrimSpace(profile.UID) != "" {
		if avatar := fetchWeiboAvatarFromMobilePage(ctx, client, profile.UID, cookies); avatar != "" {
			profile.AvatarURL = avatar
		}
	}
	if common.AccountProfileEmpty(profile) {
		return thirdparty.AccountProfile{}, fmt.Errorf("weibo profile unavailable")
	}
	return profile, nil
}

// fetchWeiboAvatarFromMobilePage fetches the user's mobile page and extracts
// the avatar URL from Open Graph meta tags.
func fetchWeiboAvatarFromMobilePage(ctx context.Context, client *http.Client, uid string, cookies map[string]string) string {
	body, err := common.FetchPageBody(ctx, common.NewHTTPClientFollow(nil),
		"https://m.weibo.cn/u/"+uid, weiboProfileHeaders("https://m.weibo.cn/"), cookies)
	if err != nil {
		return ""
	}
	// Extract og:image or avatar from the page.
	for _, pattern := range []string{
		`<meta property="og:image" content="`,
		`<meta name="twitter:image" content="`,
		`"avatar_hd":"`,
		`"avatar_large":"`,
		`"profile_image_url":"`,
	} {
		idx := strings.Index(body, pattern)
		if idx < 0 {
			continue
		}
		rest := body[idx+len(pattern):]
		if end := strings.IndexAny(rest, `"<>`); end > 0 {
			url := rest[:end]
			if strings.HasPrefix(url, "http") && strings.Contains(url, "sinaimg.cn") {
				return url
			}
		}
	}
	return ""
}

func fetchWeiboMobileConfigProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := getWeiboJSON(ctx, client, weiboMobileConfigURL, weiboProfileHeaders("https://m.weibo.cn/"), cookies, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return weiboProfileFromObject(response.Data), nil
}

func fetchWeiboSideConfigProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := getWeiboJSON(ctx, client, weiboSideConfigURL, weiboProfileHeaders("https://weibo.com/"), cookies, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return weiboProfileFromObject(response.Data), nil
}

func fetchWeiboMobileDetailProfile(ctx context.Context, client *http.Client, cookies map[string]string, uid string) (thirdparty.AccountProfile, error) {
	values := url.Values{
		"type":        {"uid"},
		"value":       {strings.TrimSpace(uid)},
		"containerid": {"100505" + strings.TrimSpace(uid)},
	}
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := getWeiboJSON(ctx, client, "https://m.weibo.cn/api/container/getIndex?"+values.Encode(), weiboProfileHeaders("https://m.weibo.cn/"), cookies, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return weiboProfileFromObject(response.Data), nil
}

func fetchWeiboAjaxProfile(ctx context.Context, client *http.Client, cookies map[string]string, uid string) (thirdparty.AccountProfile, error) {
	values := url.Values{"uid": {strings.TrimSpace(uid)}}
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := getWeiboJSON(ctx, client, "https://weibo.com/ajax/profile/info?"+values.Encode(), weiboProfileHeaders("https://weibo.com/"), cookies, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	return weiboProfileFromObject(response.Data), nil
}

func getWeiboJSON(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, cookies map[string]string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	if csrf := strings.TrimSpace(cookies["X-CSRF-TOKEN"]); csrf != "" {
		headers["x-csrf-token"] = csrf
	}
	common.ApplyHeaders(request, headers, cookies)
	// Use a redirect-following client so 302 responses from Weibo APIs are
	// handled transparently rather than treated as errors.
	followClient := &http.Client{Transport: client.Transport, Timeout: 20 * time.Second}
	response, err := followClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	common.MergeResponseCookies(cookies, response)
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return err
	}
	if target != nil && len(body) > 0 {
		if err := json.Unmarshal(body, target); err == nil {
			return nil
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("weibo profile http %d", response.StatusCode)
	}
	if target != nil {
		return fmt.Errorf("decode weibo profile response")
	}
	return nil
}

func weiboProfileHeaders(referer string) map[string]string {
	return map[string]string{
		"Accept":             "application/json, text/plain, */*",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
		"Referer":            referer,
		"User-Agent":         weiboUserAgent,
		"Sec-CH-UA":          `"Chromium";v="134", "Google Chrome";v="134", "Not?A_Brand";v="99"`,
		"Sec-CH-UA-Mobile":   "?0",
		"Sec-CH-UA-Platform": `"Windows"`,
		"Sec-Fetch-Dest":     "empty",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Site":     "same-origin",
		"DNT":                "1",
		"Sec-GPC":            "1",
		"Cache-Control":      "no-cache",
		"X-Requested-With":   "XMLHttpRequest",
	}
}

func weiboProfileFromObject(object map[string]any) thirdparty.AccountProfile {
	if len(object) == 0 {
		return thirdparty.AccountProfile{}
	}
	profile := thirdparty.AccountProfile{
		UID:      common.FirstNonEmpty(common.JSONStringValue(object["uid"]), common.JSONStringValue(object["id"]), common.JSONStringValue(object["idstr"])),
		Nickname: common.FirstNonEmpty(common.JSONStringValue(object["screen_name"]), common.JSONStringValue(object["nickname"]), common.JSONStringValue(object["name"])),
		AvatarURL: common.FirstNonEmpty(
			common.JSONStringValue(object["avatar_hd"]),
			common.JSONStringValue(object["avatar_large"]),
			common.JSONStringValue(object["profile_image_url"]),
			common.JSONStringValue(object["avatar"]),
			common.JSONStringValue(object["avatar_url"]),
			common.JSONStringValue(object["headimgurl"]),
			common.JSONStringValue(object["portrait"]),
			common.JSONStringValue(object["image"]),
			common.JSONStringValue(object["cover_image"]),
		),
	}
	for _, key := range []string{"user", "userInfo", "profile", "cardList", "card_group", "cards", "tabInfo", "newCards"} {
		if nested, ok := object[key].(map[string]any); ok {
			profile = common.MergeAccountProfiles(profile, weiboProfileFromObject(nested))
		}
		// Iterate all array elements, not just the first one.
		if arr, ok := object[key].([]any); ok {
			for _, item := range arr {
				if nested, ok := item.(map[string]any); ok {
					profile = common.MergeAccountProfiles(profile, weiboProfileFromObject(nested))
				}
			}
		}
	}
	return profile
}
