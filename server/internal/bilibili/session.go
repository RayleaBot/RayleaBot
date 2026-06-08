package bilibili

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	bilibiliUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

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

type ErrorKind string

const (
	ErrorAuth            ErrorKind = "auth"
	ErrorCSRF            ErrorKind = "csrf"
	ErrorRefresh         ErrorKind = "cookie_refresh"
	ErrorRiskControl     ErrorKind = "risk_control"
	ErrorRateLimit       ErrorKind = "rate_limit"
	ErrorSignature       ErrorKind = "signature"
	ErrorTicket          ErrorKind = "ticket"
	ErrorDevice          ErrorKind = "device"
	ErrorNotFound        ErrorKind = "not_found"
	ErrorBadRequest      ErrorKind = "bad_request"
	ErrorServer          ErrorKind = "server"
	ErrorInvalidResponse ErrorKind = "invalid_response"
	ErrorUpstream        ErrorKind = "upstream"
)

type Error struct {
	Kind       ErrorKind
	Code       int
	HTTPStatus int
	Message    string
	Err        error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{"bilibili", string(e.Kind)}
	if e.Code != 0 {
		parts = append(parts, "code "+strconv.Itoa(e.Code))
	}
	if e.HTTPStatus != 0 {
		parts = append(parts, "HTTP "+strconv.Itoa(e.HTTPStatus))
	}
	if strings.TrimSpace(e.Message) != "" {
		parts = append(parts, strings.TrimSpace(e.Message))
	}
	if e.Err != nil && strings.TrimSpace(e.Err.Error()) != "" {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, ": ")
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type PreparedCookie struct {
	Cookie    string
	Refreshed bool
	Enriched  bool
}

type SessionClient struct {
	client *http.Client
	now    func() time.Time

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

func NewSessionClient(transport http.RoundTripper, now func() time.Time) *SessionClient {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &SessionClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   defaultRequestTimeout,
		},
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

type cookieInfoResult struct {
	Refresh   bool
	Timestamp int64
}

func (c *SessionClient) fetchCookieInfo(ctx context.Context, cookie, csrf string) (cookieInfoResult, error) {
	values := url.Values{"csrf": {csrf}}
	body, _, status, err := c.send(ctx, http.MethodGet, cookieInfoURL+"?"+values.Encode(), cookie, nil)
	if err != nil {
		return cookieInfoResult{}, err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Refresh   bool `json:"refresh"`
			Timestamp any  `json:"timestamp"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return cookieInfoResult{}, &Error{Kind: ErrorInvalidResponse, HTTPStatus: status, Message: responseExcerpt(body), Err: err}
	}
	if doc.Code != 0 {
		return cookieInfoResult{}, apiError(status, doc.Code, doc.Message, body)
	}
	return cookieInfoResult{Refresh: doc.Data.Refresh, Timestamp: int64Value(doc.Data.Timestamp)}, nil
}

func (c *SessionClient) fetchRefreshCSRF(ctx context.Context, cookie string, timestamp int64) (string, error) {
	correspondPath, err := generateCorrespondPath(timestamp)
	if err != nil {
		return "", &Error{Kind: ErrorRefresh, Message: "generate correspond path", Err: err}
	}
	body, _, status, err := c.send(ctx, http.MethodGet, correspondBaseURL+correspondPath, cookie, nil)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", &Error{Kind: ErrorRefresh, HTTPStatus: status, Message: responseExcerpt(body)}
	}
	token := extractRefreshCSRF(body)
	if token == "" {
		return "", &Error{Kind: ErrorRefresh, HTTPStatus: status, Message: "refresh_csrf missing"}
	}
	return token, nil
}

func (c *SessionClient) refreshCookie(ctx context.Context, cookie, csrf, refreshCSRF, refreshToken string) (string, string, error) {
	form := url.Values{
		"csrf":          {csrf},
		"refresh_csrf":  {refreshCSRF},
		"source":        {"main_web"},
		"refresh_token": {refreshToken},
	}
	body, responseCookies, status, err := c.send(ctx, http.MethodPost, cookieRefreshURL, cookie, strings.NewReader(form.Encode()))
	if err != nil {
		return cookie, "", err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Status       int    `json:"status"`
			Message      string `json:"message"`
			RefreshToken string `json:"refresh_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return cookie, "", &Error{Kind: ErrorInvalidResponse, HTTPStatus: status, Message: responseExcerpt(body), Err: err}
	}
	if doc.Code != 0 {
		return cookie, "", apiError(status, doc.Code, doc.Message, body)
	}
	if doc.Data.Status != 0 {
		message := strings.TrimSpace(doc.Data.Message)
		if message == "" {
			message = responseExcerpt(body)
		}
		return cookie, "", &Error{Kind: ErrorRefresh, HTTPStatus: status, Message: message}
	}
	updates := map[string]string{}
	for _, item := range responseCookies {
		if strings.TrimSpace(item.Name) != "" && strings.TrimSpace(item.Value) != "" {
			updates[item.Name] = item.Value
		}
	}
	refreshed := mergeCookieValues(cookie, updates)
	return refreshed, strings.TrimSpace(doc.Data.RefreshToken), nil
}

func (c *SessionClient) confirmRefresh(ctx context.Context, cookie, csrf, oldRefreshToken string) error {
	form := url.Values{
		"csrf":          {csrf},
		"refresh_token": {oldRefreshToken},
	}
	body, _, status, err := c.send(ctx, http.MethodPost, cookieRefreshConfirmURL, cookie, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return &Error{Kind: ErrorInvalidResponse, HTTPStatus: status, Message: responseExcerpt(body), Err: err}
	}
	if doc.Code != 0 {
		return apiError(status, doc.Code, doc.Message, body)
	}
	return nil
}

func (c *SessionClient) enrichCookie(ctx context.Context, cookie string) (string, bool, error) {
	values := cookieValues(cookie)
	updates := map[string]string{}
	if strings.TrimSpace(values["buvid3"]) == "" || strings.TrimSpace(values["buvid4"]) == "" {
		device, err := c.ensureDeviceCookies(ctx, cookie)
		if err != nil {
			return cookie, false, err
		}
		if values["buvid3"] == "" && device.Buvid3 != "" {
			updates["buvid3"] = device.Buvid3
		}
		if values["buvid4"] == "" && device.Buvid4 != "" {
			updates["buvid4"] = device.Buvid4
		}
		if values["b_nut"] == "" {
			updates["b_nut"] = strconv.FormatInt(c.now().Unix(), 10)
		}
	}
	ticketExpires := int64Value(values["bili_ticket_expires"])
	if strings.TrimSpace(values["bili_ticket"]) == "" || ticketExpires <= c.now().Add(30*time.Minute).Unix() {
		ticket, err := c.ensureBiliTicket(ctx, cookie)
		if err != nil {
			return cookie, false, err
		}
		if ticket.Ticket != "" {
			updates["bili_ticket"] = ticket.Ticket
			updates["bili_ticket_expires"] = strconv.FormatInt(ticket.ExpiresAt.Unix(), 10)
		}
	}
	if len(updates) == 0 {
		return cookie, false, nil
	}
	return mergeCookieValues(cookie, updates), true, nil
}

func (c *SessionClient) ensureDeviceCookies(ctx context.Context, cookie string) (deviceCookieCache, error) {
	now := c.now()
	c.mu.Lock()
	if c.device.Buvid3 != "" && c.device.Buvid4 != "" && now.Before(c.device.ExpiresAt) {
		device := c.device
		c.mu.Unlock()
		return device, nil
	}
	c.mu.Unlock()

	body, _, status, err := c.send(ctx, http.MethodGet, buvidSPIURL, cookie, nil)
	if err != nil {
		return deviceCookieCache{}, err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Buvid3 string `json:"b_3"`
			Buvid4 string `json:"b_4"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return deviceCookieCache{}, &Error{Kind: ErrorInvalidResponse, HTTPStatus: status, Message: responseExcerpt(body), Err: err}
	}
	if doc.Code != 0 {
		return deviceCookieCache{}, apiError(status, doc.Code, doc.Message, body)
	}
	device := deviceCookieCache{
		Buvid3:    strings.TrimSpace(doc.Data.Buvid3),
		Buvid4:    strings.TrimSpace(doc.Data.Buvid4),
		ExpiresAt: now.Add(deviceCookieTTL),
	}
	if device.Buvid3 == "" && device.Buvid4 == "" {
		return deviceCookieCache{}, &Error{Kind: ErrorDevice, HTTPStatus: status, Message: "buvid missing"}
	}
	c.mu.Lock()
	c.device = device
	c.mu.Unlock()
	return device, nil
}

func (c *SessionClient) ensureBiliTicket(ctx context.Context, cookie string) (ticketCache, error) {
	now := c.now()
	c.mu.Lock()
	if c.ticket.Ticket != "" && now.Before(c.ticket.ExpiresAt.Add(-30*time.Minute)) {
		ticket := c.ticket
		c.mu.Unlock()
		return ticket, nil
	}
	c.mu.Unlock()

	ts := strconv.FormatInt(now.Unix(), 10)
	values := url.Values{
		"key_id":      {biliTicketKeyID},
		"hexsign":     {biliTicketHexSign(ts)},
		"context[ts]": {ts},
		"csrf":        {cookieValues(cookie)["bili_jct"]},
	}
	body, _, status, err := c.send(ctx, http.MethodPost, biliTicketURL+"?"+values.Encode(), cookie, nil)
	if err != nil {
		return ticketCache{}, err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Ticket    string `json:"ticket"`
			CreatedAt int64  `json:"created_at"`
			TTL       int64  `json:"ttl"`
			Nav       struct {
				Img string `json:"img"`
				Sub string `json:"sub"`
			} `json:"nav"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return ticketCache{}, &Error{Kind: ErrorInvalidResponse, HTTPStatus: status, Message: responseExcerpt(body), Err: err}
	}
	if doc.Code != 0 {
		return ticketCache{}, apiError(status, doc.Code, doc.Message, body)
	}
	expiresAt := now.Add(48 * time.Hour)
	if doc.Data.CreatedAt > 0 && doc.Data.TTL > 0 {
		expiresAt = time.Unix(doc.Data.CreatedAt+doc.Data.TTL, 0).UTC()
	}
	ticket := ticketCache{
		Ticket:    strings.TrimSpace(doc.Data.Ticket),
		ExpiresAt: expiresAt,
		WBI: wbiKeyCache{
			ImgKey:    extractWBIKey(doc.Data.Nav.Img),
			SubKey:    extractWBIKey(doc.Data.Nav.Sub),
			ExpiresAt: now.Add(wbiKeyTTL),
		},
	}
	c.mu.Lock()
	c.ticket = ticket
	if ticket.WBI.ImgKey != "" && ticket.WBI.SubKey != "" {
		c.wbi = ticket.WBI
	}
	c.mu.Unlock()
	return ticket, nil
}

func (c *SessionClient) ensureWBIKeys(ctx context.Context, cookie string) (wbiKeyCache, error) {
	now := c.now()
	c.mu.Lock()
	if c.wbi.ImgKey != "" && c.wbi.SubKey != "" && now.Before(c.wbi.ExpiresAt) {
		keys := c.wbi
		c.mu.Unlock()
		return keys, nil
	}
	c.mu.Unlock()

	if ticket, err := c.ensureBiliTicket(ctx, cookie); err == nil && ticket.WBI.ImgKey != "" && ticket.WBI.SubKey != "" {
		return ticket.WBI, nil
	}
	keys, err := c.fetchNavWBIKeys(ctx, cookie)
	if err != nil {
		return wbiKeyCache{}, err
	}
	c.mu.Lock()
	c.wbi = keys
	c.mu.Unlock()
	return keys, nil
}

func (c *SessionClient) fetchNavWBIKeys(ctx context.Context, cookie string) (wbiKeyCache, error) {
	body, _, status, err := c.send(ctx, http.MethodGet, navURL, cookie, nil)
	if err != nil {
		return wbiKeyCache{}, err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			WBIImg struct {
				ImgURL string `json:"img_url"`
				SubURL string `json:"sub_url"`
			} `json:"wbi_img"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return wbiKeyCache{}, &Error{Kind: ErrorInvalidResponse, HTTPStatus: status, Message: responseExcerpt(body), Err: err}
	}
	keys := wbiKeyCache{
		ImgKey:    extractWBIKey(doc.Data.WBIImg.ImgURL),
		SubKey:    extractWBIKey(doc.Data.WBIImg.SubURL),
		ExpiresAt: c.now().Add(wbiKeyTTL),
	}
	if keys.ImgKey != "" && keys.SubKey != "" {
		return keys, nil
	}
	if doc.Code != 0 {
		return wbiKeyCache{}, apiError(status, doc.Code, doc.Message, body)
	}
	return wbiKeyCache{}, &Error{Kind: ErrorSignature, HTTPStatus: status, Message: "WBI keys missing"}
}

func (c *SessionClient) send(ctx context.Context, method, rawURL, cookie string, body io.Reader) ([]byte, []*http.Cookie, int, error) {
	request, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, 0, err
	}
	applyBilibiliWebHeaders(request, method)
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
	request.Header.Set("Accept", "application/json, text/plain, */*")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	request.Header.Set("User-Agent", bilibiliUserAgent)
	request.Header.Set("Referer", "https://www.bilibili.com/")
	if method == http.MethodPost {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
}

func validateCookieForLogin(cookie string) error {
	if strings.TrimSpace(cookieValues(cookie)["SESSDATA"]) == "" {
		return &Error{Kind: ErrorAuth, Message: "SESSDATA missing"}
	}
	return nil
}

func apiError(httpStatus, code int, message string, body []byte) error {
	text := strings.TrimSpace(message)
	if text == "" {
		text = responseExcerpt(body)
	}
	return &Error{Kind: classifyBilibiliCode(httpStatus, code), Code: code, HTTPStatus: httpStatus, Message: text}
}

func classifyBilibiliCode(httpStatus, code int) ErrorKind {
	switch code {
	case -101, -102, -658:
		return ErrorAuth
	case -111:
		return ErrorCSRF
	case -352, 352, -412:
		return ErrorRiskControl
	case -509, -799:
		return ErrorRateLimit
	case -404:
		return ErrorNotFound
	case -400:
		return ErrorBadRequest
	case -500, -503, -504:
		return ErrorServer
	}
	return classifyHTTPStatus(httpStatus)
}

func classifyHTTPStatus(status int) ErrorKind {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrorAuth
	case http.StatusBadRequest:
		return ErrorBadRequest
	case http.StatusNotFound:
		return ErrorNotFound
	case http.StatusPreconditionFailed, http.StatusTooManyRequests:
		if status == http.StatusTooManyRequests {
			return ErrorRateLimit
		}
		return ErrorRiskControl
	default:
		if status >= 500 {
			return ErrorServer
		}
		return ErrorUpstream
	}
}

func asBilibiliError(err error) *Error {
	var target *Error
	if errors.As(err, &target) {
		return target
	}
	return nil
}

func isBilibiliAuthError(err error) bool {
	biliErr := asBilibiliError(err)
	return biliErr != nil && biliErr.Kind == ErrorAuth
}

func isBilibiliRiskControlError(err error) bool {
	biliErr := asBilibiliError(err)
	return biliErr != nil && biliErr.Kind == ErrorRiskControl
}

func isBilibiliRiskControlErrorText(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return false
	}
	return strings.Contains(text, "risk_control") || strings.Contains(text, "code -352")
}

func shouldRetryWBI(err error) bool {
	biliErr := asBilibiliError(err)
	if biliErr == nil {
		return false
	}
	return biliErr.Kind == ErrorRiskControl || biliErr.Kind == ErrorSignature || biliErr.Code == -403 || biliErr.Code == 403
}

func isBilibiliURLForWBI(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "api.bilibili.com")
}

func generateCorrespondPath(timestamp int64) (string, error) {
	block, _ := pem.Decode([]byte(correspondPublicKeyPEM))
	if block == nil {
		return "", errors.New("parse correspond public key")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	publicKey, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("correspond public key is not RSA")
	}
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, []byte("refresh_"+strconv.FormatInt(timestamp, 10)), nil)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(ciphertext), nil
}

func extractRefreshCSRF(body []byte) string {
	text := string(body)
	marker := `<div id="1-name">`
	start := strings.Index(text, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	end := strings.Index(text[start:], "</div>")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(text[start : start+end]))
}

func biliTicketHexSign(timestamp string) string {
	mac := hmac.New(sha256.New, []byte(biliTicketHMACKey))
	mac.Write([]byte("ts" + timestamp))
	return hex.EncodeToString(mac.Sum(nil))
}

func wbiMixinKey(imgKey, subKey string) string {
	raw := []byte(strings.TrimSpace(imgKey) + strings.TrimSpace(subKey))
	if len(raw) < len(wbiMixinKeyEncTab) {
		return ""
	}
	out := make([]byte, 0, 32)
	for _, index := range wbiMixinKeyEncTab {
		if index >= 0 && index < len(raw) {
			out = append(out, raw[index])
			if len(out) == 32 {
				break
			}
		}
	}
	return string(out)
}

func sanitizeWBIValue(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '!', '\'', '(', ')', '*':
			return -1
		default:
			return r
		}
	}, value)
}

func extractWBIKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Path != "" {
		value = parsed.Path
	}
	base := path.Base(value)
	if dot := strings.LastIndex(base, "."); dot > 0 {
		base = base[:dot]
	}
	return strings.TrimSpace(base)
}

func cookieValues(cookie string) map[string]string {
	values := map[string]string{}
	for _, part := range strings.Split(cookie, ";") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) != 2 {
			continue
		}
		name := strings.TrimSpace(pair[0])
		if name == "" {
			continue
		}
		values[name] = strings.TrimSpace(pair[1])
	}
	return values
}

func mergeCookieValues(cookie string, updates map[string]string) string {
	if len(updates) == 0 {
		return strings.TrimSpace(cookie)
	}
	remaining := map[string]string{}
	for key, value := range updates {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			remaining[key] = value
		}
	}
	type pair struct {
		Name  string
		Value string
	}
	pairs := []pair{}
	for _, part := range strings.Split(cookie, ";") {
		raw := strings.TrimSpace(part)
		if raw == "" {
			continue
		}
		split := strings.SplitN(raw, "=", 2)
		if len(split) != 2 || strings.TrimSpace(split[0]) == "" {
			continue
		}
		name := strings.TrimSpace(split[0])
		value := strings.TrimSpace(split[1])
		if next, ok := remaining[name]; ok {
			value = next
			delete(remaining, name)
		}
		pairs = append(pairs, pair{Name: name, Value: value})
	}
	keys := make([]string, 0, len(remaining))
	for key := range remaining {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		pairs = append(pairs, pair{Name: key, Value: remaining[key]})
	}
	parts := make([]string, 0, len(pairs))
	for _, item := range pairs {
		parts = append(parts, item.Name+"="+item.Value)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ") + ";"
}
