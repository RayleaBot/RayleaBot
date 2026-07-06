package session

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"html"
	"net/http"
	"net/url"
	"strconv"
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
