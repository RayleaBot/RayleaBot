package actions

import (
	"context"
	"encoding/base64"
	"time"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

const (
	defaultHTTPTimeoutSeconds = 10
	defaultHTTPMaxRetries     = 2
)

func init() {
	register(Metadata{
		Action:          "http.request",
		Capability:      "http.request",
		RequestSchema:   "plugin-protocol.action_http_request",
		ResponseSchema:  "plugin-protocol.local_action_result",
		AccessesNetwork: true,
		AuditFields:     []string{"plugin_id", "method", "url"},
		ErrorCodes:      commonErrorCodes("platform.invalid_request"),
	}, func(deps Deps) ActionHandler {
		return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
			return executeHTTPRequest(ctx, req.PluginID, req.Action, currentConfig(deps), deps.Capabilities)
		}
	})
}

func executeHTTPRequest(ctx context.Context, pluginID string, action pluginruntime.Action, cfg config.Config, capabilities CapabilityView) (map[string]any, error) {
	if capabilities == nil || !capabilities.CapabilityDeclared(ctx, pluginID, "http.request") {
		return nil, &pluginruntime.Error{
			Code:    "plugin.capability_violation",
			Message: "http.request capability is not declared",
		}
	}

	client := newHTTPClient(httpClientConfig{
		Timeout:           currentHTTPTimeout(cfg),
		MaxRetries:        currentHTTPMaxRetries(cfg),
		AllowPrivateHosts: append([]string(nil), cfg.HTTP.AllowPrivateHosts...),
	})
	scopeHosts := capabilities.HTTPHosts(ctx, pluginID)
	headers := cloneHTTPHeaders(action.HTTPHeaders)

	response, err := client.do(ctx, httpClientRequest{
		Method:        action.HTTPMethod,
		URL:           action.HTTPURL,
		Headers:       headers,
		Body:          append([]byte(nil), action.HTTPBody...),
		ActionTimeout: currentHTTPActionTimeout(action),
	}, scopeHosts)
	if err == errHTTPScopeViolation {
		return nil, &pluginruntime.Error{
			Code:    "plugin.capability_violation",
			Message: "http.request target is outside declared capability parameters",
		}
	}
	if err == errHTTPInvalidRequest {
		return nil, &pluginruntime.Error{
			Code:    "platform.invalid_request",
			Message: "http.request request is invalid",
		}
	}
	if err != nil {
		return nil, &pluginruntime.Error{
			Code:    "plugin.internal_error",
			Message: "http.request failed",
			Err:     err,
		}
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

func currentHTTPTimeout(cfg config.Config) time.Duration {
	seconds := cfg.HTTP.TimeoutSeconds
	if seconds <= 0 {
		seconds = defaultHTTPTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func currentHTTPMaxRetries(cfg config.Config) int {
	if cfg.HTTP.MaxRetries < 0 {
		return defaultHTTPMaxRetries
	}
	if cfg.HTTP.MaxRetries == 0 {
		return 0
	}
	return cfg.HTTP.MaxRetries
}

func currentHTTPActionTimeout(action pluginruntime.Action) time.Duration {
	if action.HTTPTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(action.HTTPTimeoutSeconds) * time.Second
}

func cloneHTTPHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}
