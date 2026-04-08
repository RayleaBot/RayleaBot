package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/pluginhttp"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

func (a *App) executeHTTPRequest(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if !a.pluginCapabilityGranted(ctx, pluginID, "http.request") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "http.request capability is not granted",
		}
	}

	scope := a.pluginGrantedScope(ctx, pluginID, "http.request")
	client := pluginhttp.New(pluginhttp.Config{
		Timeout:           currentHTTPTimeout(a.Config),
		MaxRetries:        currentHTTPMaxRetries(a.Config),
		AllowPrivateHosts: append([]string(nil), a.Config.HTTP.AllowPrivateHosts...),
	})
	response, err := client.Do(ctx, pluginhttp.Request{
		Method:        action.HTTPMethod,
		URL:           action.HTTPURL,
		Headers:       cloneHTTPHeaders(action.HTTPHeaders),
		Body:          append([]byte(nil), action.HTTPBody...),
		ActionTimeout: currentHTTPActionTimeout(action),
	}, scope.HTTPHosts)
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

func (a *App) executeSchedulerCreate(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if !a.pluginCapabilityGranted(ctx, pluginID, "scheduler.create") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "scheduler.create capability is not granted",
		}
	}
	if a == nil || a.Scheduler == nil {
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
	job, err := a.Scheduler.UpsertTask(ctx, pluginID, action.SchedulerTaskID, action.SchedulerCron, payloadBytes)
	if err != nil {
		return nil, &runtime.Error{Code: "plugin.internal_error", Message: "scheduler.create failed", Err: err}
	}
	return map[string]any{
		"task_id":  job.JobID,
		"next_run": job.NextRun.UTC().Format(time.RFC3339),
	}, nil
}

type grantedScope struct {
	HTTPHosts    []string               `json:"http_hosts"`
	StorageRoots []string               `json:"storage_roots"`
	Webhooks     []plugins.WebhookScope `json:"webhooks"`
}

func (a *App) pluginStorageRootGranted(ctx context.Context, pluginID, root string) bool {
	if strings.TrimSpace(root) == "" {
		return false
	}
	for _, grantedRoot := range a.pluginGrantedScope(ctx, pluginID, "storage.file").StorageRoots {
		if strings.TrimSpace(grantedRoot) == root {
			return true
		}
	}
	return false
}

func (a *App) pluginGrantedScope(ctx context.Context, pluginID, capability string) grantedScope {
	autoGranted := false
	if a != nil {
		for _, granted := range a.Config.Auth.AutoGrantCapabilities {
			if strings.TrimSpace(granted) == capability {
				autoGranted = true
				break
			}
		}
	}

	if a != nil && a.grantRepository != nil {
		grants, err := a.grantRepository.LoadGrants(ctx, pluginID)
		if err == nil {
			for _, grant := range grants {
				if strings.TrimSpace(grant.Capability) != capability {
					continue
				}
				scope := parseGrantedScope(grant.ScopeJSON)
				if len(scope.HTTPHosts) > 0 || len(scope.StorageRoots) > 0 || len(scope.Webhooks) > 0 {
					return scope
				}
			}
		}
	}

	if autoGranted && a != nil && a.Plugins != nil {
		if snapshot, ok := a.Plugins.Get(pluginID); ok {
			return grantedScope{
				HTTPHosts:    append([]string(nil), snapshot.ScopeHTTPHosts...),
				StorageRoots: append([]string(nil), snapshot.ScopeStorageRoots...),
				Webhooks:     append([]plugins.WebhookScope(nil), snapshot.ScopeWebhooks...),
			}
		}
	}

	return grantedScope{}
}

func parseGrantedScope(raw string) grantedScope {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return grantedScope{}
	}
	var scope grantedScope
	if err := json.Unmarshal([]byte(raw), &scope); err != nil {
		return grantedScope{}
	}
	return scope
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

func currentHTTPActionTimeout(action runtime.Action) time.Duration {
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
