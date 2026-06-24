package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// FetchAccountProfile retrieves Douyin account profile from cookies and API.
func FetchAccountProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	return FetchAccountProfileWithBrowser(ctx, client, cookies, nil)
}

// FetchAccountProfileWithBrowser fetches the Douyin account profile.
// If browserCtx is provided and the HTTP API fails, it falls back to calling
// the profile API from within the browser context, which has all security tokens
// (a_bogus, msToken) needed by the Douyin API.
func FetchAccountProfileWithBrowser(ctx context.Context, client *http.Client, cookies map[string]string, browserCtx context.Context) (thirdparty.AccountProfile, error) {
	if !HasLoginCookie(cookies) {
		return thirdparty.AccountProfile{}, fmt.Errorf("douyin cookie missing login state")
	}

	// First, try to extract profile from cookies.
	profile := extractProfileFromDouyinCookies(cookies)

	// Try the web API for richer profile data.
	apiProfile, apiErr := fetchDouyinWebProfile(ctx, client, cookies)
	if apiErr == nil {
		profile = common.MergeAccountProfiles(profile, apiProfile)
	}

	// If HTTP API failed and we have a browser context, try the browser.
	if apiErr != nil && browserCtx != nil {
		if browserProfile, err := fetchProfileFromBrowser(ctx, browserCtx); err == nil {
			profile = common.MergeAccountProfiles(profile, browserProfile)
		}
	}

	if !common.AccountProfileEmpty(profile) {
		return profile, nil
	}

	if apiErr != nil {
		return thirdparty.AccountProfile{}, fmt.Errorf("douyin profile unavailable: %w", apiErr)
	}
	return thirdparty.AccountProfile{}, fmt.Errorf("douyin profile unavailable")
}

// fetchProfileFromBrowser calls Douyin's profile API from within the browser
// context, where all security tokens (a_bogus, msToken) are available.
func fetchProfileFromBrowser(ctx context.Context, browserCtx context.Context) (thirdparty.AccountProfile, error) {
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Include aid and timestamp — the browser context has all auth cookies.
	js := `(function(){
var u = '/aweme/v1/web/user/profile/self/?aid=6383&t=' + Date.now();
return fetch(u, {credentials: 'include'}).then(function(r){ return r.text(); }).then(function(t){
return t;
}).catch(function(e){ return '{"error":"'+e.message+'"}'; });
})()`

	var raw string
	if err := chromedp.Run(runCtx, chromedp.Evaluate(js, &raw, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	})); err != nil {
		return thirdparty.AccountProfile{}, err
	}

	var check struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &check); err == nil && check.Error != "" {
		return thirdparty.AccountProfile{}, fmt.Errorf("browser profile fetch: %s", check.Error)
	}

	var response struct {
		StatusCode int `json:"status_code"`
		User       struct {
			UID          string `json:"uid"`
			ShortID      string `json:"short_id"`
			UniqueID     string `json:"unique_id"`
			Nickname     string `json:"nickname"`
			AvatarMedium struct {
				URLList []string `json:"url_list"`
			} `json:"avatar_medium"`
			AvatarThumb struct {
				URLList []string `json:"url_list"`
			} `json:"avatar_thumb"`
		} `json:"user"`
	}
	if err := json.Unmarshal([]byte(raw), &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	if response.StatusCode != 0 {
		return thirdparty.AccountProfile{}, fmt.Errorf("browser profile api status %d", response.StatusCode)
	}

	profile := thirdparty.AccountProfile{
		UID:      strings.TrimSpace(firstNonEmpty(response.User.UniqueID, response.User.ShortID, response.User.UID)),
		Nickname: strings.TrimSpace(response.User.Nickname),
	}
	if len(response.User.AvatarMedium.URLList) > 0 {
		profile.AvatarURL = strings.TrimSpace(response.User.AvatarMedium.URLList[0])
	} else if len(response.User.AvatarThumb.URLList) > 0 {
		profile.AvatarURL = strings.TrimSpace(response.User.AvatarThumb.URLList[0])
	}
	return profile, nil
}

func extractProfileFromDouyinCookies(cookies map[string]string) thirdparty.AccountProfile {
	nickname := strings.TrimSpace(cookies["nickname"])
	avatarURL := strings.TrimSpace(cookies["avatar_thumb"])
	if avatarURL == "" {
		avatarURL = strings.TrimSpace(cookies["avatar_uri"])
	}
	if nickname == "" && avatarURL == "" {
		return thirdparty.AccountProfile{}
	}
	return thirdparty.AccountProfile{
		Nickname:  nickname,
		AvatarURL: avatarURL,
	}
}

func fetchDouyinWebProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	headers := map[string]string{
		"Accept":             "application/json, text/plain, */*",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
		"Origin":             douyinOrigin,
		"Referer":            douyinReferer,
		"User-Agent":         douyinUserAgent,
		"DNT":                "1",
		"Sec-CH-UA":          `"Chromium";v="134", "Google Chrome";v="134", "Not?A_Brand";v="99"`,
		"Sec-CH-UA-Mobile":   "?0",
		"Sec-CH-UA-Platform": `"Windows"`,
		"Sec-Fetch-Dest":     "empty",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Site":     "same-origin",
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.douyin.com/aweme/v1/web/user/profile/self/?aid=6383", nil)
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	common.ApplyHeaders(request, headers, cookies)
	resp, err := client.Do(request)
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return thirdparty.AccountProfile{}, fmt.Errorf("douyin profile http %d", resp.StatusCode)
	}

	var response struct {
		StatusCode int `json:"status_code"`
		User       struct {
			UID          string `json:"uid"`
			ShortID      string `json:"short_id"`
			UniqueID     string `json:"unique_id"`
			Nickname     string `json:"nickname"`
			Signature    string `json:"signature"`
			AvatarMedium struct {
				URLList []string `json:"url_list"`
			} `json:"avatar_medium"`
			AvatarThumb struct {
				URLList []string `json:"url_list"`
			} `json:"avatar_thumb"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	if response.StatusCode != 0 {
		return thirdparty.AccountProfile{}, fmt.Errorf("douyin profile api status %d", response.StatusCode)
	}
	profile := thirdparty.AccountProfile{
		UID:      strings.TrimSpace(firstNonEmpty(response.User.UniqueID, response.User.ShortID, response.User.UID)),
		Nickname: strings.TrimSpace(response.User.Nickname),
	}
	if len(response.User.AvatarMedium.URLList) > 0 {
		profile.AvatarURL = strings.TrimSpace(response.User.AvatarMedium.URLList[0])
	} else if len(response.User.AvatarThumb.URLList) > 0 {
		profile.AvatarURL = strings.TrimSpace(response.User.AvatarThumb.URLList[0])
	}
	if common.AccountProfileEmpty(profile) {
		return thirdparty.AccountProfile{}, fmt.Errorf("douyin profile empty")
	}
	return profile, nil
}
