package httpaction

import (
	"context"
	"encoding/base64"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginhttp"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/runtime/manager"
)

type Grants interface {
	CapabilityGranted(context.Context, string, string) bool
	GrantedHTTPHosts(context.Context, string) []string
}

type Request struct {
	PluginID        string
	Action          runtimeaction.Action
	Config          config.Config
	Grants          Grants
	ThirdParty      ThirdPartyAccounts
	BilibiliSession BilibiliSession
}

func Execute(ctx context.Context, req Request) (map[string]any, error) {
	if req.Grants == nil || !req.Grants.CapabilityGranted(ctx, req.PluginID, "http.request") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "http.request capability is not granted",
		}
	}

	client := pluginhttp.New(pluginhttp.Config{
		Timeout:           currentTimeout(req.Config),
		MaxRetries:        currentMaxRetries(req.Config),
		AllowPrivateHosts: append([]string(nil), req.Config.HTTP.AllowPrivateHosts...),
	})
	scopeHosts := req.Grants.GrantedHTTPHosts(ctx, req.PluginID)
	headers := CloneHeaders(req.Action.HTTPHeaders)
	bilibiliAccount, bilibiliCookieApplied := ApplyBilibiliCookie(ctx, BilibiliCookieRequest{
		PluginID:   req.PluginID,
		RawURL:     req.Action.HTTPURL,
		ScopeHosts: scopeHosts,
		Headers:    headers,
		ThirdParty: req.ThirdParty,
		Session:    req.BilibiliSession,
	})
	requestURL := req.Action.HTTPURL
	if bilibiliCookieApplied && req.BilibiliSession != nil && isBilibiliURLForWBI(requestURL) {
		signedURL, err := req.BilibiliSession.SignURL(ctx, requestURL, headers["Cookie"])
		if err != nil {
			return nil, &runtimemanager.Error{
				Code:    "plugin.internal_error",
				Message: "http.request failed",
				Err:     err,
			}
		}
		requestURL = signedURL
	}

	response, err := client.Do(ctx, pluginhttp.Request{
		Method:        req.Action.HTTPMethod,
		URL:           requestURL,
		Headers:       headers,
		Body:          append([]byte(nil), req.Action.HTTPBody...),
		ActionTimeout: currentActionTimeout(req.Action),
	}, scopeHosts)
	if err == pluginhttp.ErrScopeViolation {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "http.request target is outside the granted scope",
		}
	}
	if err == pluginhttp.ErrInvalidRequest {
		return nil, &runtimemanager.Error{
			Code:    "platform.invalid_request",
			Message: "http.request request is invalid",
		}
	}
	if err != nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "http.request failed",
			Err:     err,
		}
	}
	if bilibiliCookieApplied {
		_ = req.ThirdParty.MarkUsed(ctx, bilibiliAccount)
	}

	result := map[string]any{
		"status_code": response.StatusCode,
		"headers":     CloneHeaders(response.Headers),
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
