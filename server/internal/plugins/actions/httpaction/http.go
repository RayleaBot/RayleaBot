package httpaction

import (
	"context"
	"encoding/base64"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	pluginhttp "github.com/RayleaBot/RayleaBot/server/internal/plugins/httpclient"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type Grants interface {
	CapabilityGranted(context.Context, string, string) bool
	GrantedHTTPHosts(context.Context, string) []string
}

type Request struct {
	PluginID           string
	Action             runtimeaction.Action
	Config             config.Config
	Grants             Grants
	CredentialInjector CredentialInjector
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
	requestURL := req.Action.HTTPURL
	var afterSuccess func(context.Context) error
	if req.CredentialInjector != nil {
		credentials, err := req.CredentialInjector.Inject(ctx, CredentialRequest{
			PluginID:   req.PluginID,
			RawURL:     req.Action.HTTPURL,
			ScopeHosts: scopeHosts,
			Headers:    headers,
		})
		if err != nil {
			return nil, &runtimemanager.Error{
				Code:    "plugin.internal_error",
				Message: "http.request failed",
				Err:     err,
			}
		}
		if credentials.URL != "" {
			requestURL = credentials.URL
		}
		afterSuccess = credentials.AfterSuccess
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
	if afterSuccess != nil {
		_ = afterSuccess(ctx)
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
