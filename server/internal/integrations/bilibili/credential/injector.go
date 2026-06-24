package credential

import (
	"context"
	"errors"
	"net/url"
	"strings"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/httpaction"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

const SubscriptionHubPluginID = "raylea.subscription-hub"

type Accounts interface {
	ListEnabled(context.Context, string) ([]thirdparty.Account, error)
	ReadCookie(context.Context, thirdparty.Account) (string, error)
	UpdateCookie(context.Context, thirdparty.Account, string) error
	MarkUsed(context.Context, thirdparty.Account) error
}

type Session interface {
	PrepareCookie(context.Context, string) (bilibilisession.PreparedCookie, error)
	SignURL(context.Context, string, string) (string, error)
}

type Injector struct {
	Accounts Accounts
	Session  Session
}

func NewInjector(accounts Accounts, session Session) *Injector {
	return &Injector{
		Accounts: accounts,
		Session:  session,
	}
}

func (i *Injector) Inject(ctx context.Context, req httpaction.CredentialRequest) (httpaction.CredentialResult, error) {
	platform := PlatformForURL(req.RawURL)
	if i == nil || i.Accounts == nil || req.Headers == nil || req.PluginID != SubscriptionHubPluginID || platform == "" || !urlHostDeclared(req.RawURL, req.ScopeHosts) || hasHeader(req.Headers, "Cookie") {
		return httpaction.CredentialResult{}, nil
	}
	accounts, err := i.Accounts.ListEnabled(ctx, platform)
	if err != nil {
		return httpaction.CredentialResult{}, nil
	}
	for _, account := range accounts {
		cookie, err := i.Accounts.ReadCookie(ctx, account)
		if err == nil && strings.TrimSpace(cookie) != "" {
			result, err := i.injectAccountCookie(ctx, req, account, cookie, platform)
			if err != nil {
				return httpaction.CredentialResult{}, err
			}
			return result, nil
		}
		if err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return httpaction.CredentialResult{}, nil
		}
	}
	return httpaction.CredentialResult{}, nil
}

func (i *Injector) injectAccountCookie(ctx context.Context, req httpaction.CredentialRequest, account thirdparty.Account, cookie string, platform string) (httpaction.CredentialResult, error) {
	if platform == thirdparty.PlatformBilibili && i.Session != nil {
		prepared, err := i.Session.PrepareCookie(ctx, cookie)
		if err != nil {
			return httpaction.CredentialResult{}, nil
		}
		if prepared.Cookie != "" {
			if prepared.Cookie != cookie && (prepared.Refreshed || prepared.Enriched) {
				_ = i.Accounts.UpdateCookie(ctx, account, prepared.Cookie)
			}
			cookie = prepared.Cookie
		}
	}
	req.Headers["Cookie"] = cookie
	result := httpaction.CredentialResult{
		AfterSuccess: func(ctx context.Context) error {
			return i.Accounts.MarkUsed(ctx, account)
		},
	}
	if platform == thirdparty.PlatformBilibili && i.Session != nil && IsBilibiliURLForWBI(req.RawURL) {
		signedURL, err := i.Session.SignURL(ctx, req.RawURL, cookie)
		if err != nil {
			return httpaction.CredentialResult{}, err
		}
		result.URL = signedURL
	}
	return result, nil
}

func PlatformForURL(rawURL string) string {
	if IsBilibiliURL(rawURL) {
		return thirdparty.PlatformBilibili
	}
	if isURLForHosts(rawURL, "weibo.com", "weibo.cn", "m.weibo.cn") {
		return thirdparty.PlatformWeibo
	}
	if isURLForHosts(rawURL, "douyin.com", "iesdouyin.com", "amemv.com") {
		return thirdparty.PlatformDouyin
	}
	if isURLForHosts(rawURL, "music.163.com", "163cn.tv") {
		return thirdparty.PlatformNeteaseMusic
	}
	return ""
}

func IsBilibiliURL(rawURL string) bool {
	return isURLForHosts(rawURL, "bilibili.com")
}

func isURLForHosts(rawURL string, roots ...string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	for _, root := range roots {
		root = strings.ToLower(strings.TrimSpace(root))
		if host == root || strings.HasSuffix(host, "."+root) {
			return true
		}
	}
	return false
}

func IsBilibiliURLForWBI(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "api.bilibili.com" || host == "api.live.bilibili.com"
}

func urlHostDeclared(rawURL string, scopeHosts []string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return false
	}
	for _, scopeHost := range scopeHosts {
		if host == strings.ToLower(strings.TrimSpace(scopeHost)) {
			return true
		}
	}
	return false
}

func hasHeader(headers map[string]string, name string) bool {
	for key, value := range headers {
		if strings.EqualFold(key, name) && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
