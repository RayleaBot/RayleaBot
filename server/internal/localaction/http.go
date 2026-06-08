package localaction

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/pluginhttp"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func (s *Service) executeHTTPRequest(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "http.request") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "http.request capability is not granted",
		}
	}

	cfg := s.config()
	client := pluginhttp.New(pluginhttp.Config{
		Timeout:           currentHTTPTimeout(cfg),
		MaxRetries:        currentHTTPMaxRetries(cfg),
		AllowPrivateHosts: append([]string(nil), cfg.HTTP.AllowPrivateHosts...),
	})
	scopeHosts := s.grants.GrantedHTTPHosts(ctx, pluginID)
	headers := cloneHTTPHeaders(action.HTTPHeaders)
	bilibiliAccount, bilibiliCookieApplied := s.applyBilibiliCookie(ctx, pluginID, action.HTTPURL, scopeHosts, headers)

	response, err := client.Do(ctx, pluginhttp.Request{
		Method:        action.HTTPMethod,
		URL:           action.HTTPURL,
		Headers:       headers,
		Body:          append([]byte(nil), action.HTTPBody...),
		ActionTimeout: currentHTTPActionTimeout(action),
	}, scopeHosts)
	if err == pluginhttp.ErrScopeViolation {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "http.request target is outside the granted scope",
		}
	}
	if err == pluginhttp.ErrInvalidRequest {
		return nil, &runtime.Error{
			Code:    "platform.invalid_request",
			Message: "http.request request is invalid",
		}
	}
	if err != nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "http.request failed",
			Err:     err,
		}
	}
	if bilibiliCookieApplied {
		_ = s.thirdParty.MarkUsed(ctx, bilibiliAccount)
	}

	result := map[string]any{
		"status_code": response.StatusCode,
		"headers":     cloneHTTPHeaders(response.Headers),
	}
	if len(response.Body) > 0 {
		if utf8.Valid(response.Body) {
			result["body_text"] = string(response.Body)
		} else {
			result["body_base64"] = base64.StdEncoding.EncodeToString(response.Body)
		}
	}
	return result, nil
}

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

func (s *Service) executeSchedulerCreate(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "scheduler.create") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "scheduler.create capability is not granted",
		}
	}
	if s.scheduler == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "scheduler engine is not available",
		}
	}

	payloadBytes, err := json.Marshal(action.SchedulerPayload)
	if err != nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "scheduler.create payload is invalid",
			Err:     err,
		}
	}
	job, err := s.scheduler.UpsertTaskWithLabel(ctx, pluginID, action.SchedulerTaskID, action.SchedulerLogLabel, action.SchedulerCron, payloadBytes)
	if err != nil {
		return nil, &runtime.Error{Code: "plugin.internal_error", Message: "scheduler.create failed", Err: err}
	}
	return map[string]any{
		"task_id":  job.JobID,
		"next_run": job.NextRun.UTC().Format(time.RFC3339),
	}, nil
}
