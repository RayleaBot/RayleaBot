package thirdpartylogin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const (
	douyinQRCodeURL   = "https://sso.douyin.com/get_qrcode/"
	douyinCheckURL    = "https://sso.douyin.com/check_qrconnect/"
	douyinCallbackURL = "https://www.douyin.com/passport/sso/login/callback/"
	douyinServiceURL  = "https://www.douyin.com/"
	douyinAid         = "10006"
	douyinHTTPMode    = "http"
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
		session, browserErr := p.createWithBrowser(ctx, now)
		if browserErr == nil {
			return session, nil
		}
		if session, err := p.createWithHTTP(ctx, now); err == nil {
			session.Values = map[string]string{"mode": douyinHTTPMode}
			return session, nil
		}
		return loginSession{}, browserErr
	}
	return p.createWithHTTP(ctx, now)
}

func (p *douyinProvider) createWithHTTP(ctx context.Context, now time.Time) (loginSession, error) {
	cookies := map[string]string{}
	result, err := createDouyinQRCode(ctx, p.client, now, cookies)
	if err != nil {
		return loginSession{}, err
	}
	return loginSession{
		Platform:  thirdparty.PlatformDouyin,
		Token:     result.Token,
		QRCodeURL: result.QRCodeURL,
		ExpiresAt: result.ExpiresAt,
		State:     StatePendingScan,
		Cookies:   cookies,
	}, nil
}

func (p *douyinProvider) Poll(ctx context.Context, session loginSession, now time.Time) (loginSession, error) {
	if p.browser != nil && session.Values["mode"] != douyinHTTPMode {
		return p.pollWithBrowser(ctx, session)
	}
	token := strings.TrimSpace(session.Token)
	if token == "" {
		return session, ErrLoginSessionNotFound
	}
	cookies := cloneStringMap(session.Cookies)
	result, err := pollDouyinQRCode(ctx, p.client, now.UTC(), token, cookies)
	if err != nil {
		return session, err
	}
	session.State = result.State
	session.Cookie = result.Cookie
	session.Account = thirdparty.AccountProfile{}
	session.Cookies = cookies
	return session, nil
}

func (p *douyinProvider) Close(session loginSession) {
	if p.browser == nil || session.Values["mode"] == douyinHTTPMode {
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

func createDouyinQRCode(ctx context.Context, client *http.Client, now time.Time, cookies map[string]string) (douyinBrowserCreateResult, error) {
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
	if _, err := getJSON(ctx, client, requestURL, douyinHeaders(), cookies, &response); err != nil {
		return douyinBrowserCreateResult{}, err
	}
	if response.ErrorCode != 0 || response.Data.ErrorCode != 0 {
		return douyinBrowserCreateResult{}, fmt.Errorf("douyin qrcode create failed: %s", firstNonEmpty(response.Data.Description, response.Description, response.Message, "invalid response"))
	}
	token := strings.TrimSpace(response.Data.Token)
	qrcodeURL := firstNonEmpty(response.Data.QRCodeIndexURL, response.Data.QRCode)
	if token == "" || qrcodeURL == "" {
		return douyinBrowserCreateResult{}, fmt.Errorf("douyin qrcode create missing token or qrcode url")
	}
	expiresAt := now.Add(3 * time.Minute)
	if response.Data.ExpireTime > 0 {
		remoteExpiresAt := time.Unix(response.Data.ExpireTime, 0).UTC()
		if remoteExpiresAt.After(now) {
			expiresAt = remoteExpiresAt
		}
	}
	return douyinBrowserCreateResult{
		Token:     token,
		QRCodeURL: qrcodeURL,
		ExpiresAt: expiresAt,
		Cookies:   cloneStringMap(cookies),
	}, nil
}

func pollDouyinQRCode(ctx context.Context, client *http.Client, now time.Time, token string, cookies map[string]string) (douyinBrowserPollResult, error) {
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
	if _, err := getJSON(ctx, client, checkURL, douyinHeaders(), cookies, &response); err != nil {
		return douyinBrowserPollResult{}, err
	}
	if response.ErrorCode != 0 {
		return douyinBrowserPollResult{}, fmt.Errorf("douyin qrcode poll failed: %s", firstNonEmpty(response.Description, response.Message, "invalid response"))
	}
	if response.Data.ErrorCode != 0 {
		return douyinBrowserPollResult{}, fmt.Errorf("douyin qrcode poll failed: %s", firstNonEmpty(response.Data.Description, "invalid response"))
	}
	switch douyinStatus(response.Data.Status) {
	case "1", "new":
		return douyinBrowserPollResult{State: StatePendingScan, Cookies: cloneStringMap(cookies)}, nil
	case "2", "scanned":
		return douyinBrowserPollResult{State: StatePendingConfirm, Cookies: cloneStringMap(cookies)}, nil
	case "3", "confirmed", "success", "succeeded":
		if err := followDouyinRedirect(ctx, client, response.Data.RedirectURL, cookies); err != nil {
			return douyinBrowserPollResult{}, err
		}
		if !douyinHasLoginCookie(cookies) {
			return douyinBrowserPollResult{}, fmt.Errorf("douyin qrcode login succeeded without login cookie")
		}
		return douyinBrowserPollResult{
			State:   StateSucceeded,
			Cookie:  cookieHeader(cookies),
			Cookies: cloneStringMap(cookies),
		}, nil
	case "4", "5", "expired", "canceled", "cancelled":
		return douyinBrowserPollResult{State: StateExpired, Cookies: cloneStringMap(cookies)}, nil
	default:
		return douyinBrowserPollResult{}, fmt.Errorf("douyin qrcode poll status %s", string(response.Data.Status))
	}
}

func followDouyinRedirect(ctx context.Context, client *http.Client, redirectURL string, cookies map[string]string) error {
	redirectURL = strings.TrimSpace(redirectURL)
	if redirectURL != "" {
		if err := followGet(ctx, client, redirectURL, douyinHeaders(), cookies); err != nil && douyinTicket(redirectURL) == "" {
			return err
		}
	}
	if douyinHasLoginCookie(cookies) {
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
	return followGet(ctx, client, callbackURL, douyinHeaders(), cookies)
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
