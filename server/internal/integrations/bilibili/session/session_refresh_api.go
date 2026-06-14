package session

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

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
