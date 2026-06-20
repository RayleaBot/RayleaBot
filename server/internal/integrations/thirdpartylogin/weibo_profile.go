package thirdpartylogin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	weiboMobileConfigURL = "https://m.weibo.cn/api/config"
	weiboSideConfigURL   = "https://weibo.com/ajax/side/config"
)

func fetchWeiboAccountProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	if !weiboHasLoginCookie(cookies) {
		return thirdparty.AccountProfile{}, fmt.Errorf("weibo cookie missing login state")
	}
	var profile thirdparty.AccountProfile
	if configProfile, err := fetchWeiboMobileConfigProfile(ctx, client, cookies); err == nil {
		profile = mergeAccountProfiles(profile, configProfile)
	}
	if accountProfileEmpty(profile) {
		if configProfile, err := fetchWeiboSideConfigProfile(ctx, client, cookies); err == nil {
			profile = mergeAccountProfiles(profile, configProfile)
		}
	}
	if strings.TrimSpace(profile.UID) != "" {
		if detailProfile, err := fetchWeiboMobileDetailProfile(ctx, client, cookies, profile.UID); err == nil {
			profile = mergeAccountProfiles(profile, detailProfile)
		}
		if detailProfile, err := fetchWeiboAjaxProfile(ctx, client, cookies, profile.UID); err == nil {
			profile = mergeAccountProfiles(profile, detailProfile)
		}
	}
	if accountProfileEmpty(profile) {
		return thirdparty.AccountProfile{}, fmt.Errorf("weibo profile unavailable")
	}
	return profile, nil
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
	applyHeaders(request, headers, cookies)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	mergeResponseCookies(cookies, response)
	body, err := io.ReadAll(io.LimitReader(response.Body, maxLoginResponseBytes))
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
		"Accept":     "application/json, text/plain, */*",
		"Referer":    referer,
		"User-Agent": weiboUserAgent,
	}
}

func weiboProfileFromObject(object map[string]any) thirdparty.AccountProfile {
	if len(object) == 0 {
		return thirdparty.AccountProfile{}
	}
	profile := thirdparty.AccountProfile{
		UID:       firstNonEmpty(jsonStringValue(object["uid"]), jsonStringValue(object["id"]), jsonStringValue(object["idstr"])),
		Nickname:  firstNonEmpty(jsonStringValue(object["screen_name"]), jsonStringValue(object["nickname"]), jsonStringValue(object["name"])),
		AvatarURL: firstNonEmpty(jsonStringValue(object["avatar_hd"]), jsonStringValue(object["avatar_large"]), jsonStringValue(object["profile_image_url"]), jsonStringValue(object["avatar"])),
	}
	for _, key := range []string{"user", "userInfo", "profile"} {
		if nested, ok := object[key].(map[string]any); ok {
			profile = mergeAccountProfiles(profile, weiboProfileFromObject(nested))
		}
	}
	return profile
}

func mergeAccountProfiles(base, next thirdparty.AccountProfile) thirdparty.AccountProfile {
	if strings.TrimSpace(base.UID) == "" {
		base.UID = strings.TrimSpace(next.UID)
	}
	if strings.TrimSpace(base.Nickname) == "" {
		base.Nickname = strings.TrimSpace(next.Nickname)
	}
	if strings.TrimSpace(base.AvatarURL) == "" {
		base.AvatarURL = strings.TrimSpace(next.AvatarURL)
	}
	return base
}
