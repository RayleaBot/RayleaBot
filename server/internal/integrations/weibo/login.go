package weibo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

const (
	weiboPassportURL = "https://passport.weibo.com"
	weiboSigninURL   = weiboPassportURL + "/sso/signin"
	weiboQRCodeURL   = weiboPassportURL + "/sso/v2/qrcode/image"
	weiboQRCheckURL  = weiboPassportURL + "/sso/v2/qrcode/check"
	weiboRedirectURL = "https://weibo.com/"
	weiboQRVersion   = "20250520"
)

var weiboUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"

type Provider struct {
	client *http.Client
}

func NewProvider(client *http.Client) *Provider {
	return &Provider{client: client}
}

func (p *Provider) Create(ctx context.Context, now time.Time) (common.LoginSession, error) {
	cookies := map[string]string{}
	headers := weiboHeaders("")
	signinURL := weiboSigninURL + "?" + url.Values{
		"entry":  {"miniblog"},
		"source": {"miniblog"},
		"url":    {weiboRedirectURL},
	}.Encode()
	if _, err := common.GetJSON(ctx, p.client, signinURL, headers, cookies, nil); err != nil {
		return common.LoginSession{}, err
	}
	csrf := strings.TrimSpace(cookies["X-CSRF-TOKEN"])
	if csrf == "" {
		return common.LoginSession{}, fmt.Errorf("weibo qrcode login missing csrf token")
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
	if _, err := common.GetJSON(ctx, p.client, qrcodeURL, weiboHeaders(csrf), cookies, &response); err != nil {
		return common.LoginSession{}, err
	}
	if response.RetCode != 20000000 || strings.TrimSpace(response.Data.QRID) == "" {
		return common.LoginSession{}, fmt.Errorf("weibo qrcode create failed: %s", common.FirstNonEmpty(response.Message, "invalid response"))
	}
	scanURL := weiboScanURL(response.Data.Image, response.Data.QRID)
	return common.LoginSession{
		Platform:  thirdparty.PlatformWeibo,
		Token:     strings.TrimSpace(response.Data.QRID),
		QRCodeURL: scanURL,
		ExpiresAt: now.Add(3 * time.Minute),
		State:     common.StatePendingScan,
		Values: map[string]string{
			"csrf": csrf,
		},
		Cookies: cookies,
	}, nil
}

func (p *Provider) Poll(ctx context.Context, session common.LoginSession, _ time.Time) (common.LoginSession, error) {
	qrid := strings.TrimSpace(session.Token)
	if qrid == "" {
		return session, common.ErrLoginSessionNotFound
	}
	cookies := common.CloneStringMap(session.Cookies)
	var response struct {
		RetCode int    `json:"retcode"`
		Message string `json:"msg"`
		Data    struct {
			URL    string `json:"url"`
			Alt    string `json:"alt"`
			UID    string `json:"uid"`
			Nick   string `json:"nickname"`
			Avatar string `json:"avatar_hd"`
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
	if _, err := common.GetJSON(ctx, p.client, checkURL, weiboHeaders(session.Values["csrf"]), cookies, &response); err != nil {
		return session, err
	}
	switch response.RetCode {
	case 20000000:
		// Use full browser-like headers when following SSO redirects so that
		// cross-domain cookies (for m.weibo.cn etc.) are properly set.
		redirectHeaders := weiboProfileHeaders("https://weibo.com/")
		if strings.TrimSpace(response.Data.URL) != "" {
			_ = common.FollowGet(ctx, p.client, response.Data.URL, redirectHeaders, cookies)
		}
		if strings.TrimSpace(response.Data.Alt) != "" {
			altURL := "https://login.sina.com.cn/sso/login.php?" + url.Values{
				"entry":      {"miniblog"},
				"alt":        {strings.TrimSpace(response.Data.Alt)},
				"returntype": {"TEXT"},
			}.Encode()
			_ = common.FollowGet(ctx, p.client, altURL, redirectHeaders, cookies)
		}
		if !weiboHasLoginCookie(cookies) {
			return session, fmt.Errorf("weibo qrcode login succeeded without login cookies")
		}
		session.State = common.StateSucceeded
		if response.Data.UID != "" {
			session.Account = thirdparty.AccountProfile{UID: response.Data.UID, Nickname: response.Data.Nick, AvatarURL: response.Data.Avatar}
		}
		if profile, err := FetchAccountProfile(ctx, p.client, cookies); err == nil {
			if session.Account.UID == "" {
				session.Account = profile
			} else {
				session.Account = common.MergeAccountProfiles(session.Account, profile)
			}
		}
		session.Cookie = common.CookieHeader(cookies)
	case 50114001:
		session.State = common.StatePendingScan
	case 50114002:
		session.State = common.StatePendingConfirm
	case 50114004:
		session.State = common.StateExpired
	default:
		message := strings.TrimSpace(response.Message)
		if strings.Contains(message, "扫") || strings.Contains(message, "scan") {
			session.State = common.StatePendingConfirm
			break
		}
		return session, fmt.Errorf("weibo qrcode poll retcode %d: %s", response.RetCode, common.FirstNonEmpty(message, "invalid response"))
	}
	session.Cookies = cookies
	return session, nil
}

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

func weiboHasLoginCookie(cookies map[string]string) bool {
	for _, name := range []string{"SUB", "SUBP"} {
		if strings.TrimSpace(cookies[name]) != "" {
			return true
		}
	}
	return false
}

func HasLoginCookie(cookies map[string]string) bool {
	return weiboHasLoginCookie(cookies)
}
