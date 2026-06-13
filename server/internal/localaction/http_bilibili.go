package localaction

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func (s *Service) applyBilibiliCookie(ctx context.Context, pluginID, rawURL string, scopeHosts []string, headers map[string]string) (thirdparty.Account, bool) {
	if s == nil || s.thirdParty == nil || pluginID != subscriptionHubPluginID || !isBilibiliURL(rawURL) || !urlHostGranted(rawURL, scopeHosts) || hasHTTPHeader(headers, "Cookie") {
		return thirdparty.Account{}, false
	}
	accounts, err := s.thirdParty.ListEnabled(ctx, thirdparty.PlatformBilibili)
	if err != nil {
		return thirdparty.Account{}, false
	}
	for _, account := range accounts {
		cookie, err := s.thirdParty.ReadCookie(ctx, account)
		if err == nil && strings.TrimSpace(cookie) != "" {
			if s.bilibiliSession != nil {
				prepared, prepareErr := s.bilibiliSession.PrepareCookie(ctx, cookie)
				if prepareErr != nil {
					return thirdparty.Account{}, false
				}
				if prepared.Cookie != "" {
					if prepared.Cookie != cookie && (prepared.Refreshed || prepared.Enriched) {
						_ = s.thirdParty.UpdateCookie(ctx, account, prepared.Cookie)
					}
					cookie = prepared.Cookie
				}
			}
			headers["Cookie"] = cookie
			return account, true
		}
		if err != nil && !errors.Is(err, secrets.ErrNotFound) {
			return thirdparty.Account{}, false
		}
	}
	return thirdparty.Account{}, false
}

func isBilibiliURL(rawURL string) bool {
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

func hasHTTPHeader(headers map[string]string, name string) bool {
	for key, value := range headers {
		if strings.EqualFold(key, name) && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
