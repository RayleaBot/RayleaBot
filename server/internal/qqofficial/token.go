package qqofficial

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Endpoints. The token service and the OpenAPI service are different hosts;
// both were confirmed against the live platform.
const (
	tokenEndpoint      = "https://bots.qq.com/app/getAppAccessToken"
	apiBase            = "https://api.sgroup.qq.com"
	sandboxAPIBase     = "https://sandbox.api.sgroup.qq.com"
	tokenRenewLeadTime = 60 * time.Second
)

// TokenSource hands out a valid app access token, refreshing it before it
// expires. The platform issues tokens with a 7200s lifetime and keeps the
// previous one valid for 60s after an early renewal, so renewing ahead of
// expiry never leaves a gap.
type TokenSource struct {
	appID     string
	appSecret string
	endpoint  string
	client    *http.Client
	now       func() time.Time

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func NewTokenSource(appID, appSecret string, client *http.Client) *TokenSource {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &TokenSource{appID: appID, appSecret: appSecret, endpoint: tokenEndpoint, client: client, now: time.Now}
}

// Token returns a cached token when it is still comfortably valid, and
// otherwise fetches a new one.
func (s *TokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && s.now().Before(s.expiresAt.Add(-tokenRenewLeadTime)) {
		return s.token, nil
	}
	token, lifetime, err := s.fetch(ctx)
	if err != nil {
		// A still-valid cached token outlives a transient refresh failure.
		if s.token != "" && s.now().Before(s.expiresAt) {
			return s.token, nil
		}
		return "", err
	}
	s.token = token
	s.expiresAt = s.now().Add(lifetime)
	return s.token, nil
}

func (s *TokenSource) fetch(ctx context.Context) (string, time.Duration, error) {
	body, err := json.Marshal(map[string]string{"appId": s.appID, "clientSecret": s.appSecret})
	if err != nil {
		return "", 0, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		return "", 0, fmt.Errorf("qqofficial: request app access token: %w", err)
	}
	defer response.Body.Close()

	var payload struct {
		AccessToken string          `json:"access_token"`
		ExpiresIn   json.RawMessage `json:"expires_in"`
		Message     string          `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return "", 0, fmt.Errorf("qqofficial: decode app access token response: %w", err)
	}
	if payload.AccessToken == "" {
		message := payload.Message
		if message == "" {
			message = "response carried no access_token"
		}
		return "", 0, fmt.Errorf("qqofficial: app access token rejected (http %d): %s", response.StatusCode, message)
	}
	return payload.AccessToken, parseExpiresIn(payload.ExpiresIn), nil
}

// parseExpiresIn accepts both the string and number spellings the platform has
// used for expires_in, and falls back to the documented 7200s lifetime.
func parseExpiresIn(raw json.RawMessage) time.Duration {
	const fallback = 7200 * time.Second
	if len(raw) == 0 {
		return fallback
	}
	var seconds int64
	if err := json.Unmarshal(raw, &seconds); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if parsed, err := strconv.ParseInt(text, 10, 64); err == nil && parsed > 0 {
			return time.Duration(parsed) * time.Second
		}
	}
	return fallback
}

// AuthorizationHeader is the scheme the platform requires on both the OpenAPI
// and the gateway Identify payload.
func AuthorizationHeader(token string) string { return "QQBot " + token }

func apiBaseURL(sandbox bool) string {
	if sandbox {
		return sandboxAPIBase
	}
	return apiBase
}
