package httpaction

import (
	"context"
	"encoding/base64"
	"time"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	pluginhttp "github.com/RayleaBot/RayleaBot/server/internal/plugins/httpclient"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

type CapabilityView interface {
	CapabilityDeclared(context.Context, string, string) bool
	HTTPHosts(context.Context, string) []string
}

type Request struct {
	PluginID     string
	Action       runtimeaction.Action
	Config       config.Config
	Capabilities CapabilityView
}

const (
	defaultTimeoutSeconds = 10
	defaultMaxRetries     = 2
)

func Execute(ctx context.Context, req Request) (map[string]any, error) {
	if req.Capabilities == nil || !req.Capabilities.CapabilityDeclared(ctx, req.PluginID, "http.request") {
		return nil, &runtimemanager.Error{
			Code:    "plugin.capability_violation",
			Message: "http.request capability is not declared",
		}
	}

	client := pluginhttp.New(pluginhttp.Config{
		Timeout:           currentTimeout(req.Config),
		MaxRetries:        currentMaxRetries(req.Config),
		AllowPrivateHosts: append([]string(nil), req.Config.HTTP.AllowPrivateHosts...),
	})
	scopeHosts := req.Capabilities.HTTPHosts(ctx, req.PluginID)
	headers := CloneHeaders(req.Action.HTTPHeaders)

	response, err := client.Do(ctx, pluginhttp.Request{
		Method:        req.Action.HTTPMethod,
		URL:           req.Action.HTTPURL,
		Headers:       headers,
		Body:          append([]byte(nil), req.Action.HTTPBody...),
		ActionTimeout: currentActionTimeout(req.Action),
	}, scopeHosts)
	if err == pluginhttp.ErrScopeViolation {
		return nil, &runtimemanager.Error{
			Code:    "plugin.capability_violation",
			Message: "http.request target is outside declared capability parameters",
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

func currentTimeout(cfg config.Config) time.Duration {
	seconds := cfg.HTTP.TimeoutSeconds
	if seconds <= 0 {
		seconds = defaultTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func currentMaxRetries(cfg config.Config) int {
	if cfg.HTTP.MaxRetries < 0 {
		return defaultMaxRetries
	}
	if cfg.HTTP.MaxRetries == 0 {
		return 0
	}
	return cfg.HTTP.MaxRetries
}

func currentActionTimeout(action runtimeaction.Action) time.Duration {
	if action.HTTPTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(action.HTTPTimeoutSeconds) * time.Second
}

func CloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}
