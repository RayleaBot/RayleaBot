package httpaction

import (
	"context"
	"errors"
	"net/url"
	"strings"

	bilibilisession "github.com/RayleaBot/RayleaBot/server/internal/bilibili/session"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

const SubscriptionHubPluginID = "raylea.subscription-hub"

type ThirdPartyAccounts interface {
	ListEnabled(context.Context, string) ([]thirdparty.Account, error)
	ReadCookie(context.Context, thirdparty.Account) (string, error)
	UpdateCookie(context.Context, thirdparty.Account, string) error
	MarkUsed(context.Context, thirdparty.Account) error
}

type BilibiliSession interface {
	PrepareCookie(context.Context, string) (bilibilisession.PreparedCookie, error)
	SignURL(context.Context, string, string) (string, error)
	InvalidateWBI()
}

type BilibiliCookieRequest struct {
	PluginID   string
	RawURL     string
	ScopeHosts []string
	Headers    map[string]string
	ThirdParty ThirdPartyAccounts
	Session    BilibiliSession
}

func ApplyBilibiliCookie(ctx context.Context, req BilibiliCookieRequest) (thirdparty.Account, bool) {
	if req.ThirdParty == nil || req.PluginID != SubscriptionHubPluginID || !IsBilibiliURL(req.RawURL) || !urlHostGranted(req.RawURL, req.ScopeHosts) || hasHeader(req.Headers, "Cookie") {
		return thirdparty.Account{}, false
	}
	accounts, err := req.ThirdParty.ListEnabled(ctx, thirdparty.PlatformBilibili)
	if err != nil {
		return thirdparty.Account{}, false
	}
	for _, account := range accounts {
		cookie, err := req.ThirdParty.ReadCookie(ctx, account)
		if err == nil && strings.TrimSpace(cookie) != "" {
			if req.Session != nil {
				prepared, prepareErr := req.Session.PrepareCookie(ctx, cookie)
				if prepareErr != nil {
					return thirdparty.Account{}, false
				}
				if prepared.Cookie != "" {
					if prepared.Cookie != cookie && (prepared.Refreshed || prepared.Enriched) {
						_ = req.ThirdParty.UpdateCookie(ctx, account, prepared.Cookie)
					}
					cookie = prepared.Cookie
				}
			}
			req.Headers["Cookie"] = cookie
			return account, true
		}
		if err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return thirdparty.Account{}, false
		}
	}
	return thirdparty.Account{}, false
}

func IsBilibiliURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host == "bilibili.com" || strings.HasSuffix(host, ".bilibili.com")
}

func isBilibiliURLForWBI(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "api.bilibili.com" || host == "api.live.bilibili.com"
}

func urlHostGranted(rawURL string, scopeHosts []string) bool {
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
