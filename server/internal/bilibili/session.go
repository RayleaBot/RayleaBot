package bilibili

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func NewSessionClient(transport http.RoundTripper, now func() time.Time, identity *IdentityProvider) *SessionClient {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if identity == nil {
		identity = NewIdentityProvider(now)
	}
	return &SessionClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   defaultRequestTimeout,
		},
		identity:      identity,
		now:           now,
		refreshChecks: make(map[string]time.Time),
	}
}

func (c *SessionClient) PrepareCookie(ctx context.Context, cookie string) (PreparedCookie, error) {
	cookie = strings.TrimSpace(cookie)
	if err := validateCookieForLogin(cookie); err != nil {
		return PreparedCookie{Cookie: cookie}, err
	}
	result := PreparedCookie{Cookie: cookie}
	if refreshed, changed, err := c.refreshCookieIfNeeded(ctx, result.Cookie); err != nil {
		return result, err
	} else if changed {
		result.Cookie = refreshed
		result.Refreshed = true
	}
	if enriched, changed, err := c.enrichCookie(ctx, result.Cookie); err == nil && changed {
		result.Cookie = enriched
		result.Enriched = true
	}
	return result, nil
}

func (c *SessionClient) SignURL(ctx context.Context, rawURL, cookie string) (string, error) {
	if !isBilibiliURLForWBI(rawURL) {
		return rawURL, nil
	}
	keys, err := c.ensureWBIKeys(ctx, cookie)
	if err != nil {
		return rawURL, err
	}
	mixinKey := wbiMixinKey(keys.ImgKey, keys.SubKey)
	if mixinKey == "" {
		return rawURL, &Error{Kind: ErrorSignature, Message: "WBI key is unavailable"}
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, err
	}
	values := parsed.Query()
	values.Del("w_rid")
	values.Set("wts", strconv.FormatInt(c.now().Unix(), 10))
	for key, list := range values {
		for index, value := range list {
			list[index] = sanitizeWBIValue(value)
		}
		values[key] = list
	}
	base := values.Encode() + mixinKey
	sum := md5.Sum([]byte(base))
	values.Set("w_rid", hex.EncodeToString(sum[:]))
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func (c *SessionClient) InvalidateWBI() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.wbi = wbiKeyCache{}
	c.ticket.WBI = wbiKeyCache{}
	c.mu.Unlock()
}
