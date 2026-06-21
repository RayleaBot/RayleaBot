package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	douyinQRCodeURL   = "https://sso.douyin.com/get_qrcode/"
	douyinCheckURL    = "https://sso.douyin.com/check_qrconnect/"
	douyinCallbackURL = "https://www.douyin.com/passport/sso/login/callback/"
	douyinServiceURL  = "https://www.douyin.com/"
	douyinAid         = "10006"
	douyinHTTPMode    = "http"

	douyinOrigin  = "https://www.douyin.com"
	douyinReferer = "https://www.douyin.com/"
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

func (p *Provider) Create(ctx context.Context, now time.Time) (common.LoginSession, error) {
	if p.browser != nil {
		session, err := p.createWithBrowser(ctx, now)
		if err == nil {
			return session, nil
		}
		// Browser failed. Douyin SSO blocks non-browser HTTP requests, so the
		// HTTP fallback will also fail. Return the browser error directly.
		return common.LoginSession{}, fmt.Errorf("douyin browser login failed (Chrome/Chromium required): %w", err)
	}
	return common.LoginSession{}, fmt.Errorf("douyin login requires Chrome/Chromium browser (configure browser_path in config)")
}

func (p *Provider) createWithHTTP(ctx context.Context, now time.Time) (common.LoginSession, error) {
	// HTTP-only fallback: visit douyin.com first to obtain session cookies,
	// then call the SSO API with those cookies (mimics browser behavior).
	cookies := map[string]string{}
	followClient := common.NewHTTPClientFollow(nil)
	_, _ = common.FetchPageBody(ctx, followClient, douyinServiceURL, douyinHeaders(), cookies)
	result, err := createDouyinQRCode(ctx, followClient, now, cookies)
	if err != nil {
		return common.LoginSession{}, fmt.Errorf("douyin http fallback: %w", err)
	}
	return common.LoginSession{
		Platform:  thirdparty.PlatformDouyin,
		Token:     result.Token,
		QRCodeURL: result.QRCodeURL,
		ExpiresAt: result.ExpiresAt,
		State:     common.StatePendingScan,
		Cookies:   cookies,
	}, nil
}

func (p *Provider) Poll(ctx context.Context, session common.LoginSession, now time.Time) (common.LoginSession, error) {
	if p.browser != nil && session.Values["mode"] != douyinHTTPMode {
		return p.pollWithBrowser(ctx, session)
	}
	token := strings.TrimSpace(session.Token)
	if token == "" {
		return session, common.ErrLoginSessionNotFound
	}
	cookies := common.CloneStringMap(session.Cookies)
	followClient := common.NewHTTPClientFollow(nil)
	result, err := pollDouyinQRCode(ctx, followClient, now.UTC(), token, cookies)
	if err != nil {
		return session, err
	}
	session.State = result.State
	session.Cookie = result.Cookie
	session.Account = thirdparty.AccountProfile{}
	session.Cookies = cookies
	return session, nil
}

func (p *Provider) Close(session common.LoginSession) {
	if p.browser == nil || session.Values["mode"] == douyinHTTPMode {
		return
	}
	p.browser.Close(session.Token)
}

func (p *Provider) createWithBrowser(ctx context.Context, now time.Time) (common.LoginSession, error) {
	result, err := p.browser.Create(ctx, now)
	if err != nil {
		return common.LoginSession{}, err
	}
	token := strings.TrimSpace(result.Token)
	qrcodeURL := strings.TrimSpace(result.QRCodeURL)
	if token == "" || qrcodeURL == "" {
		if token != "" {
			p.browser.Close(token)
		}
		return common.LoginSession{}, fmt.Errorf("douyin qrcode create invalid browser response")
	}
	expiresAt := result.ExpiresAt
	if expiresAt.IsZero() || !expiresAt.After(now) {
		expiresAt = now.Add(3 * time.Minute)
	}
	return common.LoginSession{
		Platform:  thirdparty.PlatformDouyin,
		Token:     token,
		QRCodeURL: qrcodeURL,
		ExpiresAt: expiresAt,
		State:     common.StatePendingScan,
		Cookies:   common.CloneStringMap(result.Cookies),
	}, nil
}

func (p *Provider) pollWithBrowser(ctx context.Context, session common.LoginSession) (common.LoginSession, error) {
	token := strings.TrimSpace(session.Token)
	if token == "" {
		return session, common.ErrLoginSessionNotFound
	}
	result, err := p.browser.Poll(ctx, token)
	if err != nil {
		return session, err
	}
	state := common.NormalizeState(result.State)
	if state == "" {
		state = session.State
	}
	session.State = state
	if len(result.Cookies) > 0 {
		session.Cookies = common.CloneStringMap(result.Cookies)
	}
	if state == common.StateSucceeded {
		session.Cookie = common.FirstNonEmpty(result.Cookie, common.CookieHeader(session.Cookies))
		if strings.TrimSpace(session.Cookie) == "" {
			return session, fmt.Errorf("douyin qrcode login succeeded without cookies")
		}
		if !HasLoginCookie(session.Cookies) {
			return session, fmt.Errorf("douyin qrcode login succeeded without login cookie")
		}
		session.Account = thirdparty.AccountProfile{}
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

func createDouyinQRCode(ctx context.Context, client *http.Client, now time.Time, cookies map[string]string) (BrowserCreateResult, error) {
	var response struct {
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Message     string `json:"message"`
		Data        struct {
			ErrorCode      int    `json:"error_code"`
			Description    string `json:"description"`
			QRCode         string `json:"qrcode"`
			QRCodeIndexURL string `json:"qrcode_index_url"`
			Token          string `json:"token"`
			ExpireTime     int64  `json:"expire_time"`
		} `json:"data"`
	}
	requestURL := douyinQRCodeURL + "?" + url.Values{
		"aid":       {douyinAid},
		"service":   {douyinServiceURL},
		"need_logo": {"true"},
		"t":         {fmt.Sprintf("%d", now.UnixMilli())},
	}.Encode()
	if _, err := common.GetJSON(ctx, client, requestURL, douyinHeaders(), cookies, &response); err != nil {
		return BrowserCreateResult{}, err
	}
	if response.ErrorCode != 0 || response.Data.ErrorCode != 0 {
		return BrowserCreateResult{}, fmt.Errorf("douyin qrcode create failed: %s", common.FirstNonEmpty(response.Data.Description, response.Description, response.Message, "invalid response"))
	}
	token := strings.TrimSpace(response.Data.Token)
	qrcodeURL := common.FirstNonEmpty(response.Data.QRCodeIndexURL, response.Data.QRCode)
	if token == "" || qrcodeURL == "" {
		return BrowserCreateResult{}, fmt.Errorf("douyin qrcode create missing token or qrcode url")
	}
	expiresAt := now.Add(3 * time.Minute)
	if response.Data.ExpireTime > 0 {
		remoteExpiresAt := time.Unix(response.Data.ExpireTime, 0).UTC()
		if remoteExpiresAt.After(now) {
			expiresAt = remoteExpiresAt
		}
	}
	return BrowserCreateResult{
		Token:     token,
		QRCodeURL: qrcodeURL,
		ExpiresAt: expiresAt,
		Cookies:   common.CloneStringMap(cookies),
	}, nil
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
	if _, err := common.GetJSON(ctx, client, checkURL, douyinHeaders(), cookies, &response); err != nil {
		return BrowserPollResult{}, err
	}
	if response.ErrorCode != 0 {
		return BrowserPollResult{}, fmt.Errorf("douyin qrcode poll failed: %s", common.FirstNonEmpty(response.Description, response.Message, "invalid response"))
	}
	if response.Data.ErrorCode != 0 {
		return BrowserPollResult{}, fmt.Errorf("douyin qrcode poll failed: %s", common.FirstNonEmpty(response.Data.Description, "invalid response"))
	}
	switch douyinStatus(response.Data.Status) {
	case "1", "new":
		return BrowserPollResult{State: common.StatePendingScan, Cookies: common.CloneStringMap(cookies)}, nil
	case "2", "scanned":
		return BrowserPollResult{State: common.StatePendingConfirm, Cookies: common.CloneStringMap(cookies)}, nil
	case "3", "confirmed", "success", "succeeded":
		if err := followDouyinRedirect(ctx, client, response.Data.RedirectURL, cookies); err != nil {
			return BrowserPollResult{}, err
		}
		if !HasLoginCookie(cookies) {
			return BrowserPollResult{}, fmt.Errorf("douyin qrcode login succeeded without login cookie")
		}
		return BrowserPollResult{
			State:   common.StateSucceeded,
			Cookie:  common.CookieHeader(cookies),
			Cookies: common.CloneStringMap(cookies),
		}, nil
	case "4", "5", "expired", "canceled", "cancelled":
		return BrowserPollResult{State: common.StateExpired, Cookies: common.CloneStringMap(cookies)}, nil
	default:
		return BrowserPollResult{}, fmt.Errorf("douyin qrcode poll status %s", string(response.Data.Status))
	}
}

func followDouyinRedirect(ctx context.Context, client *http.Client, redirectURL string, cookies map[string]string) error {
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL != "" {
		if err := common.FollowGet(ctx, client, redirectURL, douyinHeaders(), cookies); err != nil && douyinTicket(redirectURL) == "" {
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
	return common.FollowGet(ctx, client, callbackURL, douyinHeaders(), cookies)
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
