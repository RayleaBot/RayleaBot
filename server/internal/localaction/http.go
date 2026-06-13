package localaction

import (
	"context"
	"encoding/base64"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/pluginhttp"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
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
	requestURL := action.HTTPURL
	if bilibiliCookieApplied && s.bilibiliSession != nil && isBilibiliURLForWBI(requestURL) {
		signedURL, err := s.bilibiliSession.SignURL(ctx, requestURL, headers["Cookie"])
		if err != nil {
			return nil, &runtime.Error{
				Code:    "plugin.internal_error",
				Message: "http.request failed",
				Err:     err,
			}
		}
		requestURL = signedURL
	}

	response, err := client.Do(ctx, pluginhttp.Request{
		Method:        action.HTTPMethod,
		URL:           requestURL,
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
