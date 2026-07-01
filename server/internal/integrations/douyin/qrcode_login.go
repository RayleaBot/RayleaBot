package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	Platform          = thirdparty.PlatformDouyin
	douyinQRCodeURL   = "https://sso.douyin.com/get_qrcode/"
	douyinCheckURL    = "https://sso.douyin.com/check_qrconnect/"
	douyinCallbackURL = "https://www.douyin.com/passport/sso/login/callback/"
	douyinServiceURL  = "https://www.douyin.com/"
	douyinAid         = "10006"
	douyinHTTPMode    = "http"

	douyinOrigin  = "https://www.douyin.com"
	douyinReferer = "https://www.douyin.com/"

	douyinBrowserCreateTimeout = 25 * time.Second
)

var douyinUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"

type Provider struct {
	client  *http.Client
	browser LoginBrowser
}

type LoginBrowser interface {
	Create(context.Context, time.Time) (BrowserCreateResult, error)
	Poll(context.Context, string) (BrowserPollResult, error)
	Close(string)
}

type BrowserCreateResult struct {
	Token     string
	QRCodeURL string
	ExpiresAt time.Time
	Cookies   map[string]string
}

type BrowserPollResult struct {
	State   string
	Cookie  string
	Cookies map[string]string
}

func NewProvider(client *http.Client, browser LoginBrowser) *Provider {
	return &Provider{client: client, browser: browser}
}

func (p *Provider) Create(ctx context.Context, now time.Time) (qrcode.LoginSession, error) {
	if p.browser == nil {
		return qrcode.LoginSession{}, fmt.Errorf("douyin login requires Chrome/Chromium browser (configure browser_path in config)")
	}
	browserCtx, cancel := context.WithTimeout(ctx, douyinBrowserCreateTimeout)
	defer cancel()
	session, err := p.createWithBrowser(browserCtx, now)
	if err != nil {
		return qrcode.LoginSession{}, fmt.Errorf("douyin browser login failed (Chrome/Chromium required): %w", err)
	}
	return session, nil
}

func (p *Provider) Poll(ctx context.Context, session qrcode.LoginSession, now time.Time) (qrcode.LoginSession, error) {
	if p.browser != nil && session.Values["mode"] != douyinHTTPMode {
		return p.pollWithBrowser(ctx, session)
	}
	token := strings.TrimSpace(session.Token)
	if token == "" {
		return session, qrcode.ErrLoginSessionNotFound
	}
	cookies := thirdparty.CloneStringMap(session.Cookies)
	followClient := thirdparty.NewHTTPClientFollow(nil)
	result, err := pollDouyinQRCode(ctx, followClient, now.UTC(), token, cookies)
	if err != nil {
		return session, err
	}
	session.State = result.State
	session.Cookie = result.Cookie
	session.Account = thirdparty.AccountProfile{}
	if result.State == qrcode.StateSucceeded {
		if profile, err := FetchAccountProfile(ctx, p.client, cookies); err == nil {
			session.Account = profile
		}
	}
	session.Cookies = cookies
	return session, nil
}

func (p *Provider) Close(session qrcode.LoginSession) {
	if p.browser == nil || session.Values["mode"] == douyinHTTPMode {
		return
	}
	p.browser.Close(session.Token)
}

func (p *Provider) createWithBrowser(ctx context.Context, now time.Time) (qrcode.LoginSession, error) {
	result, err := p.browser.Create(ctx, now)
	if err != nil {
		return qrcode.LoginSession{}, err
	}
	token := strings.TrimSpace(result.Token)
	qrcodeURL := strings.TrimSpace(result.QRCodeURL)
	if token == "" || qrcodeURL == "" {
		if token != "" {
			p.browser.Close(token)
		}
		return qrcode.LoginSession{}, fmt.Errorf("douyin qrcode create invalid browser response")
	}
	expiresAt := result.ExpiresAt
	if expiresAt.IsZero() || !expiresAt.After(now) {
		expiresAt = now.Add(3 * time.Minute)
	}
	return qrcode.LoginSession{
		Platform:  thirdparty.PlatformDouyin,
		Token:     token,
		QRCodeURL: qrcodeURL,
		ExpiresAt: expiresAt,
		State:     qrcode.StatePendingScan,
		Cookies:   thirdparty.CloneStringMap(result.Cookies),
	}, nil
}

func (p *Provider) pollWithBrowser(ctx context.Context, session qrcode.LoginSession) (qrcode.LoginSession, error) {
	token := strings.TrimSpace(session.Token)
	if token == "" {
		return session, qrcode.ErrLoginSessionNotFound
	}
	result, err := p.browser.Poll(ctx, token)
	if err != nil {
		return session, err
	}
	state := qrcode.NormalizeState(result.State)
	if state == "" {
		state = session.State
	}
	session.State = state
	if len(result.Cookies) > 0 {
		session.Cookies = thirdparty.CloneStringMap(result.Cookies)
	}
	if state == qrcode.StateSucceeded {
		session.Cookie = thirdparty.FirstNonEmpty(result.Cookie, thirdparty.CookieHeader(session.Cookies))
		if strings.TrimSpace(session.Cookie) == "" {
			return session, fmt.Errorf("douyin qrcode login succeeded without cookies")
		}
		if !HasLoginCookie(session.Cookies) {
			return session, fmt.Errorf("douyin qrcode login succeeded without login cookie")
		}
		session.Account = thirdparty.AccountProfile{}
		var browserCtx context.Context
		if cb, ok := p.browser.(*ChromedpBrowser); ok {
			browserCtx = cb.SessionContext(session.Token)
		}
		if profile, err := FetchAccountProfileWithBrowser(ctx, p.client, session.Cookies, browserCtx); err == nil {
			session.Account = profile
		}
	}
	return session, nil
}

func douyinHeaders() map[string]string {
	return map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"Origin":     douyinOrigin,
		"Referer":    douyinReferer,
		"User-Agent": douyinUserAgent,
	}
}

func pollDouyinQRCode(ctx context.Context, client *http.Client, now time.Time, token string, cookies map[string]string) (BrowserPollResult, error) {
	var response struct {
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Message     string `json:"message"`
		Data        struct {
			ErrorCode   int             `json:"error_code"`
			Description string          `json:"description"`
			Status      json.RawMessage `json:"status"`
			RedirectURL string          `json:"redirect_url"`
		} `json:"data"`
	}
	checkURL := douyinCheckURL + "?" + url.Values{
		"aid":     {douyinAid},
		"service": {douyinServiceURL},
		"token":   {strings.TrimSpace(token)},
		"t":       {fmt.Sprintf("%d", now.UnixMilli())},
	}.Encode()
	if _, err := thirdparty.GetJSON(ctx, client, checkURL, douyinHeaders(), cookies, &response); err != nil {
		return BrowserPollResult{}, err
	}
	if response.ErrorCode != 0 {
		return BrowserPollResult{}, fmt.Errorf("douyin qrcode poll failed: %s", thirdparty.FirstNonEmpty(response.Description, response.Message, "invalid response"))
	}
	if response.Data.ErrorCode != 0 {
		return BrowserPollResult{}, fmt.Errorf("douyin qrcode poll failed: %s", thirdparty.FirstNonEmpty(response.Data.Description, "invalid response"))
	}
	switch douyinStatus(response.Data.Status) {
	case "1", "new":
		return BrowserPollResult{State: qrcode.StatePendingScan, Cookies: thirdparty.CloneStringMap(cookies)}, nil
	case "2", "scanned":
		return BrowserPollResult{State: qrcode.StatePendingConfirm, Cookies: thirdparty.CloneStringMap(cookies)}, nil
	case "3", "confirmed", "success", "succeeded":
		if err := followDouyinRedirect(ctx, client, response.Data.RedirectURL, cookies); err != nil {
			return BrowserPollResult{}, err
		}
		if !HasLoginCookie(cookies) {
			return BrowserPollResult{}, fmt.Errorf("douyin qrcode login succeeded without login cookie")
		}
		return BrowserPollResult{
			State:   qrcode.StateSucceeded,
			Cookie:  thirdparty.CookieHeader(cookies),
			Cookies: thirdparty.CloneStringMap(cookies),
		}, nil
	case "4", "5", "expired", "canceled", "cancelled":
		return BrowserPollResult{State: qrcode.StateExpired, Cookies: thirdparty.CloneStringMap(cookies)}, nil
	default:
		return BrowserPollResult{}, fmt.Errorf("douyin qrcode poll status %s", string(response.Data.Status))
	}
}

func followDouyinRedirect(ctx context.Context, client *http.Client, redirectURL string, cookies map[string]string) error {
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL != "" {
		if err := thirdparty.FollowGet(ctx, client, redirectURL, douyinHeaders(), cookies); err != nil && douyinTicket(redirectURL) == "" {
			return err
		}
	}
	if HasLoginCookie(cookies) {
		return nil
	}
	ticket := douyinTicket(redirectURL)
	if ticket == "" {
		return fmt.Errorf("douyin qrcode login missing ticket")
	}
	callbackURL := douyinCallbackURL + "?" + url.Values{
		"next":   {douyinServiceURL},
		"ticket": {ticket},
	}.Encode()
	return thirdparty.FollowGet(ctx, client, callbackURL, douyinHeaders(), cookies)
}

func douyinStatus(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(strings.ToLower(text))
	}
	var number int
	if err := json.Unmarshal(raw, &number); err == nil {
		return fmt.Sprintf("%d", number)
	}
	return strings.Trim(strings.ToLower(value), `"`)
}

func douyinTicket(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Query().Get("ticket"))
}

func HasLoginCookie(cookies map[string]string) bool {
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
