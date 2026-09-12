package actions

import (
	"context"
	"encoding/base64"
	"time"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func httpRequestRegistrar() registrar {
	return registrar{
		kind: "http.request",
		factory: func(deps Deps) ActionHandler {
			return func(ctx context.Context, req ActionRequest) (map[string]any, error) {
				return executeHTTPRequest(ctx, req.PluginID, req.Action, currentConfig(deps), deps.Permissions)
			}
		},
	}
}

func executeHTTPRequest(ctx context.Context, pluginID string, action plugins.Action, cfg config.Config, permissions PermissionView) (map[string]any, error) {
	if permissions == nil || !permissions.PermissionDeclared(ctx, pluginID, "http.request") {
		return nil, &plugins.Error{
			Code:    errorcodes.PluginPermissionDenied,
			Message: "http.request permission is not declared",
		}
	}

	client := newHTTPClient(httpClientConfig{
		Timeout:              currentHTTPTimeout(cfg),
		MaxRetries:           currentHTTPMaxRetries(cfg),
		MaxResponseBodyBytes: currentHTTPMaxResponseBodyBytes(cfg),
		AllowPrivateHosts:    append([]string(nil), cfg.HTTP.AllowPrivateHosts...),
	})
	headers := cloneHTTPHeaders(action.HTTPHeaders)

	response, err := client.do(ctx, httpClientRequest{
		Method:        action.HTTPMethod,
		URL:           action.HTTPURL,
		Headers:       headers,
		Body:          append([]byte(nil), action.HTTPBody...),
		ActionTimeout: currentHTTPActionTimeout(action),
	})
	if err == errHTTPInvalidRequest {
		return nil, &plugins.Error{
			Code:    errorcodes.PlatformInvalidRequest,
			Message: "http.request request is invalid",
		}
	}
	if err == errHTTPResponseTooLarge {
		return nil, &plugins.Error{
			Code:    errorcodes.PlatformUpstreamResponseTooLarge,
			Message: "http.request response exceeded resource limits",
			Err:     err,
		}
	}
	if err != nil {
		return nil, &plugins.Error{
			Code:    errorcodes.PluginInternalError,
			Message: "http.request failed",
			Err:     err,
		}
	}

	result := map[string]any{
		"status_code": response.StatusCode,
		"headers":     cloneHTTPHeaders(response.Headers),
		"set_cookies": append([]string{}, response.SetCookies...),
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
		seconds = config.DefaultHTTPConfig().TimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func currentHTTPMaxRetries(cfg config.Config) int {
	if cfg.HTTP.MaxRetries < 0 {
		return config.DefaultHTTPConfig().MaxRetries
	}
	if cfg.HTTP.MaxRetries == 0 {
		return 0
	}
	return cfg.HTTP.MaxRetries
}

func currentHTTPMaxResponseBodyBytes(cfg config.Config) int64 {
	if cfg.HTTP.MaxResponseBodyBytes <= 0 {
		return defaultHTTPMaxResponseBodyBytes
	}
	return cfg.HTTP.MaxResponseBodyBytes
}

func currentHTTPActionTimeout(action plugins.Action) time.Duration {
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
