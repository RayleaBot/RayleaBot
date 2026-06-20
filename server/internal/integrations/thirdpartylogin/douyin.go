package thirdpartylogin

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	douyinQRCodeURL   = "https://sso.douyin.com/get_qrcode/?need_logo=true"
	douyinCheckURL    = "https://sso.douyin.com/check_qrconnect/"
	douyinCallbackURL = "https://www.douyin.com/passport/sso/login/callback/"
)

type douyinProvider struct {
	client  *http.Client
	browser douyinLoginBrowser
}

type douyinLoginBrowser interface {
	Create(context.Context, time.Time) (douyinBrowserCreateResult, error)
	Poll(context.Context, string) (douyinBrowserPollResult, error)
	Close(string)
}

type douyinBrowserCreateResult struct {
	Token     string
	QRCodeURL string
	ExpiresAt time.Time
	Cookies   map[string]string
}

type douyinBrowserPollResult struct {
	State   string
	Cookie  string
	Cookies map[string]string
}

func newDouyinProvider(client *http.Client, browser douyinLoginBrowser) *douyinProvider {
	return &douyinProvider{client: client, browser: browser}
}

func (p *douyinProvider) Create(ctx context.Context, now time.Time) (loginSession, error) {
	if p.browser != nil {
		return p.createWithBrowser(ctx, now)
	}
	cookies := map[string]string{}
	var response struct {
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Message     string `json:"message"`
		Data        struct {
			QRCode    string `json:"qrcode"`
			QRCodeURL string `json:"qrcode_index_url"`
			Token     string `json:"token"`
		} `json:"data"`
	}
	if _, err := getJSON(ctx, p.client, douyinQRCodeURL, douyinHeaders(), cookies, &response); err != nil {
		return loginSession{}, err
	}
	if response.ErrorCode != 0 || strings.TrimSpace(response.Data.Token) == "" {
		return loginSession{}, fmt.Errorf("douyin qrcode create failed: %s", firstNonEmpty(response.Description, response.Message, "invalid response"))
	}
	qrcodeURL := firstNonEmpty(response.Data.QRCode, response.Data.QRCodeURL, douyinCheckURL+"?token="+url.QueryEscape(response.Data.Token))
	return loginSession{
		Platform:  thirdparty.PlatformDouyin,
		Token:     strings.TrimSpace(response.Data.Token),
		QRCodeURL: qrcodeURL,
		ExpiresAt: now.Add(3 * time.Minute),
		State:     StatePendingScan,
		Cookies:   cookies,
	}, nil
}

func (p *douyinProvider) Poll(ctx context.Context, session loginSession, _ time.Time) (loginSession, error) {
	if p.browser != nil {
		return p.pollWithBrowser(ctx, session)
	}
	token := strings.TrimSpace(session.Token)
	if token == "" {
		return session, ErrLoginSessionNotFound
	}
	cookies := cloneStringMap(session.Cookies)
	var response struct {
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Message     string `json:"message"`
		Data        struct {
			ErrorCode   int    `json:"error_code"`
			Description string `json:"description"`
			Status      int    `json:"status"`
			RedirectURL string `json:"redirect_url"`
		} `json:"data"`
	}
	checkURL := douyinCheckURL + "?" + url.Values{"token": {token}}.Encode()
	if _, err := getJSON(ctx, p.client, checkURL, douyinHeaders(), cookies, &response); err != nil {
		return session, err
	}
	if response.ErrorCode != 0 {
		return session, fmt.Errorf("douyin qrcode poll failed: %s", firstNonEmpty(response.Description, response.Message, "invalid response"))
	}
	if response.Data.ErrorCode != 0 {
		return session, fmt.Errorf("douyin qrcode poll failed: %s", firstNonEmpty(response.Data.Description, "invalid response"))
	}
	switch response.Data.Status {
	case 1:
		session.State = StatePendingScan
	case 2:
		session.State = StatePendingConfirm
	case 3:
		ticket := douyinTicket(response.Data.RedirectURL)
		if ticket == "" {
			return session, fmt.Errorf("douyin qrcode login missing ticket")
		}
		callbackURL := douyinCallbackURL + "?" + url.Values{
			"next":   {"https://www.douyin.com"},
			"ticket": {ticket},
		}.Encode()
		if err := followGet(ctx, p.client, callbackURL, douyinHeaders(), cookies); err != nil {
			return session, err
		}
		if !douyinHasLoginCookie(cookies) {
			return session, fmt.Errorf("douyin qrcode login succeeded without login cookie")
		}
		session.State = StateSucceeded
		session.Cookie = cookieHeader(cookies)
		session.Account = thirdparty.AccountProfile{}
	case 4, 5:
		session.State = StateExpired
	default:
		return session, fmt.Errorf("douyin qrcode poll status %d", response.Data.Status)
	}
	session.Cookies = cookies
	return session, nil
}

func (p *douyinProvider) Close(session loginSession) {
	if p.browser == nil {
		return
	}
	p.browser.Close(session.Token)
}

func (p *douyinProvider) createWithBrowser(ctx context.Context, now time.Time) (loginSession, error) {
	result, err := p.browser.Create(ctx, now)
	if err != nil {
		return loginSession{}, err
	}
	token := strings.TrimSpace(result.Token)
	qrcodeURL := strings.TrimSpace(result.QRCodeURL)
	if token == "" || qrcodeURL == "" {
		if token != "" {
			p.browser.Close(token)
		}
		return loginSession{}, fmt.Errorf("douyin qrcode create invalid browser response")
	}
	expiresAt := result.ExpiresAt
	if expiresAt.IsZero() || !expiresAt.After(now) {
		expiresAt = now.Add(3 * time.Minute)
	}
	return loginSession{
		Platform:  thirdparty.PlatformDouyin,
		Token:     token,
		QRCodeURL: qrcodeURL,
		ExpiresAt: expiresAt,
		State:     StatePendingScan,
		Cookies:   cloneStringMap(result.Cookies),
	}, nil
}

func (p *douyinProvider) pollWithBrowser(ctx context.Context, session loginSession) (loginSession, error) {
	token := strings.TrimSpace(session.Token)
	if token == "" {
		return session, ErrLoginSessionNotFound
	}
	result, err := p.browser.Poll(ctx, token)
	if err != nil {
		return session, err
	}
	state := normalizeState(result.State)
	if state == "" {
		state = session.State
	}
	session.State = state
	if len(result.Cookies) > 0 {
		session.Cookies = cloneStringMap(result.Cookies)
	}
	if state == StateSucceeded {
		session.Cookie = firstNonEmpty(result.Cookie, cookieHeader(session.Cookies))
		if strings.TrimSpace(session.Cookie) == "" {
			return session, fmt.Errorf("douyin qrcode login succeeded without cookies")
		}
		if !douyinHasLoginCookie(session.Cookies) {
			return session, fmt.Errorf("douyin qrcode login succeeded without login cookie")
		}
		session.Account = thirdparty.AccountProfile{}
	}
	return session, nil
}

func douyinHeaders() map[string]string {
	return map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"Origin":     "https://www.douyin.com",
		"Referer":    "https://www.douyin.com/",
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	}
}

func douyinTicket(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get("ticket"))
}

func douyinHasLoginCookie(cookies map[string]string) bool {
	for _, name := range []string{
		"sessionid",
		"sid_guard",
		"sid_tt",
		"uid_tt",
		"uid_tt_ss",
		"passport_auth_status",
		"passport_auth_status_ss",
	} {
		if strings.TrimSpace(cookies[name]) != "" {
			return true
		}
	}
	return false
}
