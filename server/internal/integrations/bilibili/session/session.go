package session

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	cookieInfoURL           = "https://passport.bilibili.com/x/passport-login/web/cookie/info"
	cookieRefreshURL        = "https://passport.bilibili.com/x/passport-login/web/cookie/refresh"
	cookieRefreshConfirmURL = "https://passport.bilibili.com/x/passport-login/web/confirm/refresh"
	correspondBaseURL       = "https://www.bilibili.com/correspond/1/"
	biliTicketURL           = "https://api.bilibili.com/bapis/bilibili.api.ticket.v1.Ticket/GenWebTicket"
	buvidSPIURL             = "https://api.bilibili.com/x/frontend/finger/spi"

	biliTicketKeyID   = "ec02"
	biliTicketHMACKey = "XgwSnGZ1p"

	refreshCheckInterval = 6 * time.Hour
	wbiKeyTTL            = 12 * time.Hour
	deviceCookieTTL      = 24 * time.Hour
)

const correspondPublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDLgd2OAkcGVtoE3ThUREbio0Eg
Uc/prcajMKXvkCKFCWhJYJcLkcM2DKKcSeFpD/j6Boy538YXnR6VhcuUJOhH2x71
nzPjfdTcqMz7djHum0qSZA0AyCBDABUqCrfNgCiJ00Ra7GmRj+YCK1NJEuewlb40
JNrRuoEUXpabUzGB8QIDAQAB
-----END PUBLIC KEY-----`

var wbiMixinKeyEncTab = []int{
	46, 47, 18, 2, 53, 8, 23, 32, 15, 50, 10, 31, 58, 3, 45, 35,
	27, 43, 5, 49, 33, 9, 42, 19, 29, 28, 14, 39, 12, 38, 41, 13,
	37, 48, 7, 16, 24, 55, 40, 61, 26, 17, 0, 1, 60, 51, 30, 4,
	22, 25, 54, 21, 56, 59, 6, 63, 57, 62, 11, 36, 20, 34, 44, 52,
}

type PreparedCookie struct {
	Cookie    string
	Refreshed bool
	Enriched  bool
}

type SessionClient struct {
	client   *http.Client
	identity *IdentityProvider
	now      func() time.Time

	mu            sync.Mutex
	refreshChecks map[string]time.Time
	wbi           wbiKeyCache
	ticket        ticketCache
	device        deviceCookieCache
}

type wbiKeyCache struct {
	ImgKey    string
	SubKey    string
	ExpiresAt time.Time
}

type ticketCache struct {
	Ticket    string
	ExpiresAt time.Time
	WBI       wbiKeyCache
}

type deviceCookieCache struct {
	Buvid3    string
	Buvid4    string
	ExpiresAt time.Time
}

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
			Timeout:   DefaultRequestTimeout,
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
	c.mu.Lock()
	c.wbi = wbiKeyCache{}
	c.ticket.WBI = wbiKeyCache{}
	c.mu.Unlock()
}

func (c *SessionClient) send(ctx context.Context, method, rawURL, cookie string, body io.Reader) ([]byte, []*http.Cookie, int, error) {
	request, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, 0, err
	}
	c.identity.ApplyHeaders(request, method)
	if strings.TrimSpace(cookie) != "" {
		request.Header.Set("Cookie", strings.TrimSpace(cookie))
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, nil, 0, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, nil, response.StatusCode, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return responseBody, response.Cookies(), response.StatusCode, &Error{Kind: classifyHTTPStatus(response.StatusCode), HTTPStatus: response.StatusCode, Message: responseExcerpt(responseBody)}
	}
	return responseBody, response.Cookies(), response.StatusCode, nil
}

func (c *SessionClient) shouldCheckRefresh(fingerprint string) bool {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	checkedAt, ok := c.refreshChecks[fingerprint]
	return !ok || now.Sub(checkedAt) >= refreshCheckInterval
}

func (c *SessionClient) rememberRefreshCheck(fingerprint string) {
	c.mu.Lock()
	c.refreshChecks[fingerprint] = c.now()
	c.mu.Unlock()
}

func applyBilibiliWebHeaders(request *http.Request, method string) {
	defaultIdentity := NewIdentityProvider(nil)
	defaultIdentity.ApplyHeaders(request, method)
}
