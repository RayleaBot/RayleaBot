package localaction

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"
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
	response, err := client.Do(ctx, pluginhttp.Request{
		Method:        action.HTTPMethod,
		URL:           action.HTTPURL,
		Headers:       cloneHTTPHeaders(action.HTTPHeaders),
		Body:          append([]byte(nil), action.HTTPBody...),
		ActionTimeout: currentHTTPActionTimeout(action),
	}, s.grants.GrantedHTTPHosts(ctx, pluginID))
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
	job, err := s.scheduler.UpsertTask(ctx, pluginID, action.SchedulerTaskID, action.SchedulerCron, payloadBytes)
	if err != nil {
		return nil, &runtime.Error{Code: "plugin.internal_error", Message: "scheduler.create failed", Err: err}
	}
	return map[string]any{
		"task_id":  job.JobID,
		"next_run": job.NextRun.UTC().Format(time.RFC3339),
	}, nil
}
