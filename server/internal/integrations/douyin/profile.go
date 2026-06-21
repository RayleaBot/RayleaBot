package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

// FetchAccountProfile attempts to retrieve Douyin account profile from cookies.
// Douyin does not expose a straightforward profile API without additional signatures,
// so we extract what we can from the available endpoints.
func FetchAccountProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	if !HasLoginCookie(cookies) {
		return thirdparty.AccountProfile{}, fmt.Errorf("douyin cookie missing login state")
	}

	// Try to extract basic info from cookie values
	profile := extractProfileFromDouyinCookies(cookies)
	if !common.AccountProfileEmpty(profile) {
		return profile, nil
	}

	// Try fetching user info from douyin.com API
	if apiProfile, err := fetchDouyinWebProfile(ctx, client, cookies); err == nil {
		if !common.AccountProfileEmpty(apiProfile) {
			return apiProfile, nil
		}
	}

	return thirdparty.AccountProfile{}, fmt.Errorf("douyin profile unavailable")
}

func extractProfileFromDouyinCookies(cookies map[string]string) thirdparty.AccountProfile {
	uid := strings.TrimSpace(cookies["uid_tt"])
	if uid == "" {
		uid = strings.TrimSpace(cookies["uid_tt_ss"])
	}
	nickname := strings.TrimSpace(cookies["nickname"])
	avatarURL := strings.TrimSpace(cookies["avatar_thumb"])
	if avatarURL == "" {
		avatarURL = strings.TrimSpace(cookies["avatar_uri"])
	}

	if uid == "" && nickname == "" && avatarURL == "" {
		return thirdparty.AccountProfile{}
	}
	return thirdparty.AccountProfile{
		UID:       uid,
		Nickname:  nickname,
		AvatarURL: avatarURL,
	}
}

func fetchDouyinWebProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	// Try the douyin.com user info endpoint
	var response struct {
		StatusCode int `json:"status_code"`
		User       struct {
			UID      string `json:"uid"`
			Nickname string `json:"nickname"`
			Avatar   struct {
				URLList []string `json:"url_list"`
			} `json:"avatar_medium"`
		} `json:"user"`
	}
	headers := map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"Origin":     douyinOrigin,
		"Referer":    douyinReferer,
		"User-Agent": douyinUserAgent,
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.douyin.com/aweme/v1/web/user/profile/self/", nil)
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
	if err := json.Unmarshal(body, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	if response.StatusCode != 0 {
		return thirdparty.AccountProfile{}, fmt.Errorf("douyin profile api status %d", response.StatusCode)
	}
	profile := thirdparty.AccountProfile{
		UID:      strings.TrimSpace(response.User.UID),
		Nickname: strings.TrimSpace(response.User.Nickname),
	}
	if len(response.User.Avatar.URLList) > 0 {
		profile.AvatarURL = strings.TrimSpace(response.User.Avatar.URLList[0])
	}
	if common.AccountProfileEmpty(profile) {
		return thirdparty.AccountProfile{}, fmt.Errorf("douyin profile empty")
	}
	return profile, nil
}
