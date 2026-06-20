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
	weiboPassportURL = "https://passport.weibo.com"
	weiboSigninURL   = weiboPassportURL + "/sso/signin"
	weiboQRCodeURL   = weiboPassportURL + "/sso/v2/qrcode/image"
	weiboQRCheckURL  = weiboPassportURL + "/sso/v2/qrcode/check"
	weiboRedirectURL = "https://weibo.com/"
	weiboQRVersion   = "20250520"
)

type weiboProvider struct {
	client *http.Client
}

func newWeiboProvider(client *http.Client) *weiboProvider {
	return &weiboProvider{client: client}
}

func (p *weiboProvider) Create(ctx context.Context, now time.Time) (loginSession, error) {
	cookies := map[string]string{}
	headers := weiboHeaders("")
	signinURL := weiboSigninURL + "?" + url.Values{
		"entry":  {"miniblog"},
		"source": {"miniblog"},
		"url":    {weiboRedirectURL},
	}.Encode()
	if _, err := getJSON(ctx, p.client, signinURL, headers, cookies, nil); err != nil {
		return loginSession{}, err
	}
	csrf := strings.TrimSpace(cookies["X-CSRF-TOKEN"])
	if csrf == "" {
		return loginSession{}, fmt.Errorf("weibo qrcode login missing csrf token")
	}
	var response struct {
		RetCode int    `json:"retcode"`
		Message string `json:"msg"`
		Data    struct {
			QRID  string `json:"qrid"`
			Image string `json:"image"`
		} `json:"data"`
	}
	qrcodeURL := weiboQRCodeURL + "?" + url.Values{"entry": {"miniblog"}, "size": {"180"}}.Encode()
	if _, err := getJSON(ctx, p.client, qrcodeURL, weiboHeaders(csrf), cookies, &response); err != nil {
		return loginSession{}, err
	}
	if response.RetCode != 20000000 || strings.TrimSpace(response.Data.QRID) == "" {
		return loginSession{}, fmt.Errorf("weibo qrcode create failed: %s", firstNonEmpty(response.Message, "invalid response"))
	}
	scanURL := weiboScanURL(response.Data.Image, response.Data.QRID)
	return loginSession{
		Platform:  thirdparty.PlatformWeibo,
		Token:     strings.TrimSpace(response.Data.QRID),
		QRCodeURL: scanURL,
		ExpiresAt: now.Add(3 * time.Minute),
		State:     StatePendingScan,
		Values: map[string]string{
			"csrf": csrf,
		},
		Cookies: cookies,
	}, nil
}

func (p *weiboProvider) Poll(ctx context.Context, session loginSession, _ time.Time) (loginSession, error) {
	qrid := strings.TrimSpace(session.Token)
	if qrid == "" {
		return session, ErrLoginSessionNotFound
	}
	cookies := cloneStringMap(session.Cookies)
	var response struct {
		RetCode int    `json:"retcode"`
		Message string `json:"msg"`
		Data    struct {
			URL string `json:"url"`
			Alt string `json:"alt"`
		} `json:"data"`
	}
	checkURL := weiboQRCheckURL + "?" + url.Values{
		"entry":  {"miniblog"},
		"source": {"miniblog"},
		"url":    {weiboRedirectURL},
		"qrid":   {qrid},
		"rid":    {""},
		"ver":    {weiboQRVersion},
	}.Encode()
	if _, err := getJSON(ctx, p.client, checkURL, weiboHeaders(session.Values["csrf"]), cookies, &response); err != nil {
		return session, err
	}
	switch response.RetCode {
	case 20000000:
		if strings.TrimSpace(response.Data.URL) != "" {
			if err := followGet(ctx, p.client, response.Data.URL, map[string]string{"User-Agent": weiboUserAgent}, cookies); err != nil {
				return session, err
			}
		}
		if strings.TrimSpace(response.Data.Alt) != "" {
			altURL := "https://login.sina.com.cn/sso/login.php?" + url.Values{
				"entry":      {"miniblog"},
				"alt":        {strings.TrimSpace(response.Data.Alt)},
				"returntype": {"TEXT"},
			}.Encode()
			if err := followGet(ctx, p.client, altURL, map[string]string{"User-Agent": weiboUserAgent}, cookies); err != nil {
				return session, err
			}
		}
		session.State = StateSucceeded
		session.Cookie = cookieHeader(cookies)
		session.Account = thirdparty.AccountProfile{}
	case 50114001:
		session.State = StatePendingScan
	case 50114002:
		session.State = StatePendingConfirm
	case 50114004:
		session.State = StateExpired
	default:
		message := strings.TrimSpace(response.Message)
		if strings.Contains(message, "扫") || strings.Contains(message, "scan") {
			session.State = StatePendingConfirm
			break
		}
		return session, fmt.Errorf("weibo qrcode poll retcode %d: %s", response.RetCode, firstNonEmpty(message, "invalid response"))
	}
	session.Cookies = cookies
	return session, nil
}

const weiboUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

func weiboHeaders(csrf string) map[string]string {
	headers := map[string]string{
		"Accept":     "application/json, text/plain, */*",
		"Origin":     weiboPassportURL,
		"Referer":    weiboPassportURL + "/sso/signin?entry=miniblog&source=miniblog&url=https://weibo.com/",
		"User-Agent": weiboUserAgent,
	}
	if strings.TrimSpace(csrf) != "" {
		headers["x-csrf-token"] = strings.TrimSpace(csrf)
	}
	return headers
}

func weiboScanURL(imageURL, qrid string) string {
	parsed, err := url.Parse(strings.TrimSpace(imageURL))
	if err == nil {
		if value := strings.TrimSpace(parsed.Query().Get("data")); value != "" {
			return value
		}
	}
	return "https://passport.weibo.cn/signin/qrcode/scan?qr=" + url.QueryEscape(strings.TrimSpace(qrid))
}
