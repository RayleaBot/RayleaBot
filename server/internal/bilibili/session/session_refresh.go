package session

import (
	"context"
	"strings"
)

func (c *SessionClient) refreshCookieIfNeeded(ctx context.Context, cookie string) (string, bool, error) {
	values := cookieValues(cookie)
	csrf := strings.TrimSpace(values["bili_jct"])
	refreshToken := strings.TrimSpace(values["ac_time_value"])
	if csrf == "" || refreshToken == "" {
		return cookie, false, nil
	}
	fingerprint := cookieFingerprint(cookie)
	if !c.shouldCheckRefresh(fingerprint) {
		return cookie, false, nil
	}
	info, err := c.fetchCookieInfo(ctx, cookie, csrf)
	if err != nil {
		if !isBilibiliAuthError(err) {
			return cookie, false, nil
		}
		return cookie, false, err
	}
	c.rememberRefreshCheck(fingerprint)
	if !info.Refresh {
		return cookie, false, nil
	}
	timestamp := info.Timestamp
	if timestamp < 1_000_000_000_000 {
		timestamp = c.now().UnixMilli()
	}
	refreshCSRF, err := c.fetchRefreshCSRF(ctx, cookie, timestamp)
	if err != nil {
		return cookie, false, err
	}
	refreshed, newRefreshToken, err := c.refreshCookie(ctx, cookie, csrf, refreshCSRF, refreshToken)
	if err != nil {
		return cookie, false, err
	}
	if newRefreshToken != "" {
		refreshed = mergeCookieValues(refreshed, map[string]string{"ac_time_value": newRefreshToken})
	}
	_ = c.confirmRefresh(ctx, refreshed, csrf, refreshToken)
	c.rememberRefreshCheck(cookieFingerprint(refreshed))
	return refreshed, true, nil
}
