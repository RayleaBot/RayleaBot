package thirdpartylogin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	neteaseQRCodeKeyURL   = "https://music.163.com/weapi/login/qrcode/unikey?csrf_token="
	neteaseQRCodeCheckURL = "https://music.163.com/weapi/login/qrcode/client/login?csrf_token="
)

type neteaseMusicProvider struct {
	client *http.Client
}

func newNeteaseMusicProvider(client *http.Client) *neteaseMusicProvider {
	return &neteaseMusicProvider{client: client}
}

func (p *neteaseMusicProvider) Create(ctx context.Context, now time.Time) (loginSession, error) {
	cookies := map[string]string{}
	var response struct {
		Code   int    `json:"code"`
		UniKey string `json:"unikey"`
	}
	form, err := neteaseWEAPIForm(`{"type":1}`)
	if err != nil {
		return loginSession{}, err
	}
	if _, err := postFormJSON(ctx, p.client, neteaseQRCodeKeyURL, form, neteaseHeaders(), cookies, &response); err != nil {
		return loginSession{}, err
	}
	if response.Code != 200 || strings.TrimSpace(response.UniKey) == "" {
		return loginSession{}, fmt.Errorf("netease music qrcode create code %d", response.Code)
	}
	key := strings.TrimSpace(response.UniKey)
	return loginSession{
		Platform:  thirdparty.PlatformNeteaseMusic,
		Token:     key,
		QRCodeURL: "https://music.163.com/login?codekey=" + url.QueryEscape(key),
		ExpiresAt: now.Add(3 * time.Minute),
		State:     StatePendingScan,
		Cookies:   cookies,
	}, nil
}

func (p *neteaseMusicProvider) Poll(ctx context.Context, session loginSession, _ time.Time) (loginSession, error) {
	key := strings.TrimSpace(session.Token)
	if key == "" {
		return session, ErrLoginSessionNotFound
	}
	cookies := cloneStringMap(session.Cookies)
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Cookie  string `json:"cookie"`
		Account struct {
			ID int64 `json:"id"`
		} `json:"account"`
		Profile struct {
			UserID    int64  `json:"userId"`
			Nickname  string `json:"nickname"`
			AvatarURL string `json:"avatarUrl"`
		} `json:"profile"`
	}
	form, err := neteaseWEAPIForm(`{"type":1,"key":"` + escapeJSONString(key) + `"}`)
	if err != nil {
		return session, err
	}
	if _, err := postFormJSON(ctx, p.client, neteaseQRCodeCheckURL, form, neteaseHeaders(), cookies, &response); err != nil {
		return session, err
	}
	switch response.Code {
	case 801:
		session.State = StatePendingScan
	case 802:
		session.State = StatePendingConfirm
	case 803:
		session.State = StateSucceeded
		session.Cookie = firstNonEmpty(response.Cookie, cookieHeader(cookies))
		session.Account = neteaseProfile(response)
	case 800:
		session.State = StateExpired
	default:
		return session, fmt.Errorf("netease music qrcode poll code %d: %s", response.Code, strings.TrimSpace(response.Message))
	}
	session.Cookies = cookies
	return session, nil
}

func neteaseHeaders() map[string]string {
	return map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"Origin":     "https://music.163.com",
		"Referer":    "https://music.163.com/",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	}
}

func neteaseProfile(response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Cookie  string `json:"cookie"`
	Account struct {
		ID int64 `json:"id"`
	} `json:"account"`
	Profile struct {
		UserID    int64  `json:"userId"`
		Nickname  string `json:"nickname"`
		AvatarURL string `json:"avatarUrl"`
	} `json:"profile"`
}) thirdparty.AccountProfile {
	uid := response.Profile.UserID
	if uid == 0 {
		uid = response.Account.ID
	}
	if uid == 0 && strings.TrimSpace(response.Profile.Nickname) == "" && strings.TrimSpace(response.Profile.AvatarURL) == "" {
		return thirdparty.AccountProfile{}
	}
	return thirdparty.AccountProfile{
		UID:       strconv.FormatInt(uid, 10),
		Nickname:  strings.TrimSpace(response.Profile.Nickname),
		AvatarURL: strings.TrimSpace(response.Profile.AvatarURL),
	}
}
