package weibo

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

const (
	weiboMobileConfigURL = "https://m.weibo.cn/api/config"
	weiboSideConfigURL   = "https://weibo.com/ajax/side/config"
)

func FetchAccountProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	if !weiboHasLoginCookie(cookies) {
		return thirdparty.AccountProfile{}, weiboCredentialExpiredError(0, 0)
	}
	var profile thirdparty.AccountProfile
	probeErrors := make([]error, 0, 2)
	if configProfile, err := fetchWeiboMobileConfigProfile(ctx, client, cookies); err == nil {
		profile = thirdparty.MergeAccountProfiles(profile, configProfile)
	} else {
		probeErrors = append(probeErrors, err)
	}
	if configProfile, err := fetchWeiboSideConfigProfile(ctx, client, cookies); err == nil {
		profile = thirdparty.MergeAccountProfiles(profile, configProfile)
	} else {
		probeErrors = append(probeErrors, err)
	}
	for _, probeErr := range probeErrors {
		if typed := thirdparty.AsThirdPartyError(probeErr); typed != nil && (typed.Kind == thirdparty.ErrorAuth || typed.Kind == thirdparty.ErrorExpired) {
			return thirdparty.AccountProfile{}, probeErr
		}
	}
	if thirdparty.AccountProfileEmpty(profile) {
		if len(probeErrors) > 0 {
			return thirdparty.AccountProfile{}, probeErrors[0]
		}
		return thirdparty.AccountProfile{}, thirdparty.NewPlatformError("weibo", thirdparty.ErrorInvalidResponse, 0, 0, "微博账号资料不可用", nil)
	}
	if strings.TrimSpace(profile.UID) != "" {
		_ = thirdparty.FollowGet(ctx, client, "https://m.weibo.cn/", weiboProfileHeaders("https://m.weibo.cn/"), cookies)
		if detailProfile, err := fetchWeiboMobileDetailProfile(ctx, client, cookies, profile.UID); err == nil {
			profile = thirdparty.MergeAccountProfiles(profile, detailProfile)
		}
		if detailProfile, err := fetchWeiboAjaxProfile(ctx, client, cookies, profile.UID); err == nil {
			profile = thirdparty.MergeAccountProfiles(profile, detailProfile)
		}
	}
	if strings.TrimSpace(profile.AvatarURL) == "" && strings.TrimSpace(profile.UID) != "" {
		if avatar := fetchWeiboAvatarFromMobilePage(ctx, client, profile.UID, cookies); avatar != "" {
			profile.AvatarURL = avatar
		}
	}
	return profile, nil
}

// fetchWeiboAvatarFromMobilePage fetches the user's mobile page and extracts
// the avatar URL from Open Graph meta tags.
func fetchWeiboAvatarFromMobilePage(ctx context.Context, client *http.Client, uid string, cookies map[string]string) string {
	body, err := thirdparty.FetchPageBody(ctx, weiboFollowClient(client),
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
			candidate := rest[:end]
			if parsed, err := url.Parse(candidate); err == nil && parsed.Scheme == "https" && thirdparty.HostMatches(parsed.Hostname(), "sinaimg.cn") {
				return candidate
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
	if login, exists := response.Data["login"].(bool); exists && !login {
		return thirdparty.AccountProfile{}, weiboCredentialExpiredError(0, http.StatusOK)
	}
	return weiboProfileFromObject(response.Data), nil
}

func fetchWeiboSideConfigProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	var response struct {
		OK   int            `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := getWeiboJSON(ctx, client, weiboSideConfigURL, weiboProfileHeaders("https://weibo.com/"), cookies, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	if response.OK == -100 {
		return thirdparty.AccountProfile{}, weiboCredentialExpiredError(response.OK, http.StatusOK)
	}
	if response.OK != 0 && response.OK != 1 {
		return thirdparty.AccountProfile{}, thirdparty.NewPlatformError("weibo", thirdparty.ErrorUpstream, response.OK, http.StatusOK, "微博账号检查被上游拒绝", nil)
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
	if csrf := strings.TrimSpace(cookies["X-CSRF-TOKEN"]); csrf != "" {
		headers["x-csrf-token"] = csrf
	}
	response, err := thirdparty.GetJSON(ctx, weiboFollowClient(client), rawURL, headers, cookies, target)
	if err != nil {
		if response != nil && (response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices) {
			return thirdparty.NewPlatformError("weibo", thirdparty.ClassifyHTTPStatus(response.StatusCode), 0, response.StatusCode, "微博账号检查被上游拒绝", nil)
		}
		if response != nil {
			return thirdparty.NewPlatformError("weibo", thirdparty.ErrorInvalidResponse, 0, response.StatusCode, "微博账号检查响应格式不正确", err)
		}
		return thirdparty.NewPlatformError("weibo", thirdparty.ErrorNetwork, 0, 0, "微博账号检查请求失败", err)
	}
	return nil
}

func weiboFollowClient(client *http.Client) *http.Client {
	if client == nil {
		return thirdparty.NewHTTPClientFollow(nil)
	}
	return thirdparty.NewHTTPClientFollow(client.Transport)
}

func weiboCredentialExpiredError(code, httpStatus int) error {
	return thirdparty.NewPlatformError("weibo", thirdparty.ErrorAuth, code, httpStatus, "微博账号 CK 已失效", nil)
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
		UID:      thirdparty.FirstNonEmpty(thirdparty.JSONStringValue(object["uid"]), thirdparty.JSONStringValue(object["id"]), thirdparty.JSONStringValue(object["idstr"])),
		Nickname: thirdparty.FirstNonEmpty(thirdparty.JSONStringValue(object["screen_name"]), thirdparty.JSONStringValue(object["nickname"]), thirdparty.JSONStringValue(object["name"])),
		AvatarURL: thirdparty.FirstNonEmpty(
			thirdparty.JSONStringValue(object["avatar_hd"]),
			thirdparty.JSONStringValue(object["avatar_large"]),
			thirdparty.JSONStringValue(object["profile_image_url"]),
			thirdparty.JSONStringValue(object["avatar"]),
			thirdparty.JSONStringValue(object["avatar_url"]),
			thirdparty.JSONStringValue(object["headimgurl"]),
			thirdparty.JSONStringValue(object["portrait"]),
			thirdparty.JSONStringValue(object["image"]),
			thirdparty.JSONStringValue(object["cover_image"]),
		),
	}
	for _, key := range []string{"user", "userInfo", "profile", "cardList", "card_group", "cards", "tabInfo", "newCards"} {
		if nested, ok := object[key].(map[string]any); ok {
			profile = thirdparty.MergeAccountProfiles(profile, weiboProfileFromObject(nested))
		}
		// Iterate all array elements, not just the first one.
		if arr, ok := object[key].([]any); ok {
			for _, item := range arr {
				if nested, ok := item.(map[string]any); ok {
					profile = thirdparty.MergeAccountProfiles(profile, weiboProfileFromObject(nested))
				}
			}
		}
	}
	return profile
}
