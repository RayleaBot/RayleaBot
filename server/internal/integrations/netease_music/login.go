package netease_music

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

const (
	neteaseQRCodeCheckURL = "https://music.163.com/weapi/login/qrcode/client/login?csrf_token="
	neteaseAccountURL     = "https://music.163.com/weapi/w/nuser/account/get?csrf_token="
)

var neteaseUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"

type Provider struct {
	client     *http.Client
	deviceOnce sync.Once
	deviceID   string
}

func NewProvider(client *http.Client) *Provider {
	return &Provider{client: client}
}

func (p *Provider) ensureDeviceID() string {
	p.deviceOnce.Do(func() {
		id, err := neteaseDeviceID()
		if err != nil {
			id = fmt.Sprintf("%x", time.Now().UnixNano())
		}
		p.deviceID = id
	})
	return p.deviceID
}

// Create initiates a NetEase Music QR code login session.
// Create initiates a NetEase Music QR code login session.
// The flow mirrors the official web client: first visit the home page to
// obtain __csrf and other session cookies, then call the unikey WEAPI.
func (p *Provider) Create(ctx context.Context, now time.Time) (common.LoginSession, error) {
	deviceID := p.ensureDeviceID()
	// Complete tracking cookies matching the official web client fingerprint.
	// Reference: gmg137/netease-cloud-music-gtk (Rust, actively maintained 2025).
	nuid, _ := randomHex(16)
	nnid := fmt.Sprintf("%s,%d", nuid, now.UnixMilli())
	nmtid, _ := randomHex(16)
	wnmcid, _ := randomHex(16)
	cookies := map[string]string{
		"os":            "pc",
		"appver":        "2.7.1.198277",
		"osver":         "10",
		"deviceId":      deviceID,
		"WEVNSM":        "1.0.0",
		"WNMCID":        wnmcid,
		"_ntes_nnid":    nnid,
		"_ntes_nuid":    nuid,
		"NMTID":         nmtid,
		"__remember_me": "true",
		"channel":       "",
	}
	// Visit the home page to obtain __csrf and session cookies.
	followClient := common.NewHTTPClientFollow(nil)
	_, _ = common.FetchPageBody(ctx, followClient, "https://music.163.com/", neteaseHeaders(), cookies)
	csrf := strings.TrimSpace(cookies["__csrf"])

	var response struct {
		Code   int    `json:"code"`
		UniKey string `json:"unikey"`
	}
	form, err := neteaseWEAPIFormPayload(map[string]any{
		"type":       1,
		"csrf_token": csrf,
	})
	if err != nil {
		return common.LoginSession{}, err
	}
	unikeyURL := "https://music.163.com/weapi/login/qrcode/unikey?csrf_token=" + url.QueryEscape(csrf)
	if _, err := common.PostFormJSON(ctx, p.client, unikeyURL, form, neteaseHeaders(), cookies, &response); err != nil {
		return common.LoginSession{}, err
	}
	if response.Code != 200 || strings.TrimSpace(response.UniKey) == "" {
		return common.LoginSession{}, fmt.Errorf("netease music qrcode create code %d", response.Code)
	}
	key := strings.TrimSpace(response.UniKey)
	qrcodeURL := "https://music.163.com/login?" + url.Values{
		"codekey": {key},
		"chainId": {neteaseChainID(deviceID, now)},
	}.Encode()
	return common.LoginSession{
		Platform:  thirdparty.PlatformNeteaseMusic,
		Token:     key,
		QRCodeURL: qrcodeURL,
		ExpiresAt: now.Add(3 * time.Minute),
		State:     common.StatePendingScan,
		Cookies:   cookies,
	}, nil
}

func (p *Provider) Poll(ctx context.Context, session common.LoginSession, _ time.Time) (common.LoginSession, error) {
	key := strings.TrimSpace(session.Token)
	if key == "" {
		return session, common.ErrLoginSessionNotFound
	}
	cookies := common.CloneStringMap(session.Cookies)
	if strings.TrimSpace(cookies["os"]) == "" {
		cookies["os"] = "pc"
	}
	var response neteaseLoginResponse
	// csrf_token may be empty during polling (before login); may appear after 803.
	form, err := neteaseWEAPIFormPayload(map[string]any{
		"type":       1,
		"key":        key,
		"csrf_token": strings.TrimSpace(cookies["__csrf"]),
	})
	if err != nil {
		return session, err
	}
	if _, err := common.PostFormJSON(ctx, p.client, neteaseQRCodeCheckURL, form, neteaseHeaders(), cookies, &response); err != nil {
		return session, err
	}
	switch response.Code {
	case 801:
		session.State = common.StatePendingScan
	case 802:
		session.State = common.StatePendingConfirm
		if profile := neteaseProfile(response); !common.AccountProfileEmpty(profile) {
			session.Account = profile
		}
	case 803:
		session.State = common.StateSucceeded
		for key, value := range common.CookieMapFromHeader(response.Cookie) {
			cookies[key] = value
		}
		session.Cookie = common.FirstNonEmpty(response.Cookie, common.CookieHeader(cookies))
		if strings.TrimSpace(session.Cookie) == "" {
			return session, fmt.Errorf("netease music qrcode login succeeded without cookies")
		}
		profile := neteaseProfile(response)
		if common.AccountProfileEmpty(profile) {
			profile = session.Account
		}
		if common.AccountProfileEmpty(profile) {
			if fetched, err := fetchNeteaseAccountProfile(ctx, p.client, cookies); err == nil {
				profile = fetched
			}
		}
		session.Account = profile
	case 800:
		session.State = common.StateExpired
	default:
		return session, fmt.Errorf("netease music qrcode poll code %d: %s", response.Code, strings.TrimSpace(response.Message))
	}
	session.Cookies = cookies
	return session, nil
}

func neteaseHeaders() map[string]string {
	return map[string]string{
		"Accept":             "application/json, text/plain, */*",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
		"Origin":             "https://music.163.com",
		"Referer":            "https://music.163.com/",
		"User-Agent":         neteaseUserAgent,
		"Sec-CH-UA":          `"Chromium";v="134", "Google Chrome";v="134", "Not?A_Brand";v="99"`,
		"Sec-CH-UA-Mobile":   "?0",
		"Sec-CH-UA-Platform": `"Windows"`,
		"Sec-Fetch-Dest":     "empty",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Site":     "same-site",
		"DNT":                "1",
		"Sec-GPC":            "1",
		"Cache-Control":      "no-cache",
		"X-Real-IP":          "211.161.244.70",
	}
}

// randomHex generates n random bytes as a lowercase hex string.
func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func neteaseDeviceID() (string, error) {
	var bytes [26]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(bytes[:])), nil
}

// neteaseUUID generates a random UUID v4 string used as the request key.
func neteaseUUID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:]), nil
}

func neteaseChainID(deviceID string, now time.Time) string {
	return fmt.Sprintf("v1_%s_web_login_%d", strings.TrimSpace(deviceID), now.UnixMilli())
}

type neteaseLoginResponse struct {
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

func neteaseProfile(response neteaseLoginResponse) thirdparty.AccountProfile {
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

func neteaseWEAPIFormPayload(payload map[string]any) (url.Values, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return neteaseWEAPIForm(string(encoded))
}

func fetchNeteaseAccountProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	form, err := neteaseWEAPIFormPayload(map[string]any{
		"csrf_token": strings.TrimSpace(cookies["__csrf"]),
	})
	if err != nil {
		return thirdparty.AccountProfile{}, err
	}
	var response neteaseLoginResponse
	if _, err := common.PostFormJSON(ctx, client, neteaseAccountURL, form, neteaseHeaders(), cookies, &response); err != nil {
		return thirdparty.AccountProfile{}, err
	}
	profile := neteaseProfile(response)
	if common.AccountProfileEmpty(profile) {
		return thirdparty.AccountProfile{}, fmt.Errorf("netease music profile unavailable")
	}
	return profile, nil
}

func HasLoginCookie(cookies map[string]string) bool {
	return strings.TrimSpace(cookies["MUSIC_U"]) != ""
}

// FetchAccountProfile retrieves the NetEase Music account profile from cookies.
func FetchAccountProfile(ctx context.Context, client *http.Client, cookies map[string]string) (thirdparty.AccountProfile, error) {
	return fetchNeteaseAccountProfile(ctx, client, cookies)
}
