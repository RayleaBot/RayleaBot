package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"rayleabot/server/internal/config"
	"rayleabot/server/internal/permission"
	"rayleabot/server/internal/pluginfile"
	"rayleabot/server/internal/pluginhttp"
	"rayleabot/server/internal/pluginkv"
	"rayleabot/server/internal/runtime"
)

const (
	defaultPluginLogRateLimit   = "200/10s"
	defaultKVValueMaxBytes      = 65536
	defaultKVTotalLimitMegabyte = 16
	defaultFileMaxBytes         = 10 * 1024 * 1024
	defaultPluginWorkdirMB      = 256
	defaultHTTPTimeoutSeconds   = 10
	defaultHTTPMaxRetries       = 2
)

type pluginLogLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	limit   permission.RateLimit
	records map[string][]time.Time
}

func newPluginLogLimiter(cfg config.Config) *pluginLogLimiter {
	return &pluginLogLimiter{
		now:     time.Now,
		limit:   parsePluginLogRateLimit(cfg),
		records: make(map[string][]time.Time),
	}
}

func (l *pluginLogLimiter) SetLimit(limit permission.RateLimit) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limit = limit
	if len(l.records) == 0 {
		return
	}
	now := l.now().UTC()
	for pluginID, entries := range l.records {
		l.records[pluginID] = prunePluginLogEntries(entries, now, l.limit.Window)
		if len(l.records[pluginID]) == 0 {
			delete(l.records, pluginID)
		}
	}
}

func (l *pluginLogLimiter) Allow(pluginID string) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().UTC()
	entries := prunePluginLogEntries(l.records[pluginID], now, l.limit.Window)
	if len(entries) >= l.limit.Count {
		l.records[pluginID] = entries
		return false
	}
	l.records[pluginID] = append(entries, now)
	return true
}

func prunePluginLogEntries(entries []time.Time, now time.Time, window time.Duration) []time.Time {
	if window <= 0 {
		return nil
	}
	cutoff := now.Add(-window)
	index := 0
	for index < len(entries) && entries[index].Before(cutoff) {
		index++
	}
	return append([]time.Time(nil), entries[index:]...)
}

func parsePluginLogRateLimit(cfg config.Config) permission.RateLimit {
	limit, err := permission.ParseRateLimit(strings.TrimSpace(cfg.Logging.RateLimitPerPlugin))
	if err == nil {
		return limit
	}
	limit, _ = permission.ParseRateLimit(defaultPluginLogRateLimit)
	return limit
}

func (a *App) executeLocalAction(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
	switch action.Kind {
	case "logger.write":
		return a.executeLoggerWrite(ctx, pluginID, requestID, action)
	case "storage.kv":
		return a.executeStorageKV(ctx, pluginID, action)
	case "storage.file":
		return a.executeStorageFile(ctx, pluginID, action)
	case "http.request":
		return a.executeHTTPRequest(ctx, pluginID, action)
	default:
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported local action kind",
		}
	}
}

func (a *App) executeLoggerWrite(ctx context.Context, pluginID, requestID string, action runtime.Action) (map[string]any, error) {
	if !a.pluginCapabilityGranted(ctx, pluginID, "logger.write") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "logger.write capability is not granted",
		}
	}
	if a.pluginLogLimiter != nil && !a.pluginLogLimiter.Allow(pluginID) {
		return nil, &runtime.Error{
			Code:    "platform.rate_limited",
			Message: "plugin log throughput exceeded the configured platform limit",
		}
	}
	if a == nil || a.Logger == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "logger.write is not available",
		}
	}

	level := strings.TrimSpace(action.LogLevel)
	message := a.redactString(action.LogMessage)
	attrs := []any{
		"component", "plugin",
		"plugin_id", pluginID,
		"request_id", requestID,
	}
	keys := make([]string, 0, len(action.LogFields))
	for key := range action.LogFields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attrs = append(attrs, key, redactValue(a.redactText, action.LogFields[key]))
	}

	switch level {
	case "debug":
		a.Logger.Debug(message, attrs...)
	case "warn":
		a.Logger.Warn(message, attrs...)
	case "error":
		a.Logger.Error(message, attrs...)
	default:
		a.Logger.Info(message, attrs...)
	}
	return map[string]any{}, nil
}

func (a *App) executeStorageKV(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if !a.pluginCapabilityGranted(ctx, pluginID, "storage.kv") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "storage.kv capability is not granted",
		}
	}
	if a == nil || a.pluginKV == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "storage.kv repository is not available",
		}
	}

	switch action.StorageOperation {
	case "get":
		value, exists, err := a.pluginKV.Get(ctx, pluginID, action.StorageKey)
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.kv get failed", Err: err}
		}
		result := map[string]any{
			"key":    action.StorageKey,
			"exists": exists,
		}
		if exists {
			result["value"] = value
		}
		return result, nil
	case "set":
		err := a.pluginKV.Set(ctx, pluginID, action.StorageKey, action.StorageValue, currentKVLimits(a.Config))
		if errors.Is(err, pluginkv.ErrValueTooLarge) || errors.Is(err, pluginkv.ErrQuotaExceeded) {
			return nil, &runtime.Error{Code: "platform.value_too_large", Message: "storage.kv value exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.kv set failed", Err: err}
		}
		return map[string]any{}, nil
	case "delete":
		deleted, err := a.pluginKV.Delete(ctx, pluginID, action.StorageKey)
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.kv delete failed", Err: err}
		}
		return map[string]any{
			"key":     action.StorageKey,
			"deleted": deleted,
		}, nil
	case "list":
		keys, err := a.pluginKV.List(ctx, pluginID, action.StoragePrefix)
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.kv list failed", Err: err}
		}
		return map[string]any{
			"prefix": action.StoragePrefix,
			"keys":   keys,
		}, nil
	default:
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported storage.kv operation",
		}
	}
}

func (a *App) executeStorageFile(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if !a.pluginCapabilityGranted(ctx, pluginID, "storage.file") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "storage.file capability is not granted",
		}
	}
	if !a.pluginStorageRootGranted(ctx, pluginID, action.StorageRoot) {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "storage.file root is outside the granted scope",
		}
	}
	if a == nil || a.pluginFiles == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "storage.file service is not available",
		}
	}

	switch action.StorageOperation {
	case "read":
		result, err := a.pluginFiles.Read(pluginID, action.StoragePath)
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.file read failed", Err: err}
		}
		payload := map[string]any{
			"root":   action.StorageRoot,
			"path":   action.StoragePath,
			"exists": result.Exists,
		}
		if result.Exists {
			if result.IsText {
				payload["content_text"] = string(result.Content)
			} else {
				payload["content_base64"] = base64.StdEncoding.EncodeToString(result.Content)
			}
		}
		return payload, nil
	case "write":
		err := a.pluginFiles.Write(pluginID, action.StoragePath, action.StorageContent, currentFileLimits(a.Config))
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if errors.Is(err, pluginfile.ErrFileTooLarge) || errors.Is(err, pluginfile.ErrQuotaExceeded) {
			return nil, &runtime.Error{Code: "platform.value_too_large", Message: "storage.file write exceeds configured platform limit"}
		}
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.file write failed", Err: err}
		}
		return map[string]any{
			"root": action.StorageRoot,
			"path": action.StoragePath,
		}, nil
	case "delete":
		deleted, err := a.pluginFiles.Delete(pluginID, action.StoragePath)
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.file delete failed", Err: err}
		}
		return map[string]any{
			"root":    action.StorageRoot,
			"path":    action.StoragePath,
			"deleted": deleted,
		}, nil
	case "list":
		paths, err := a.pluginFiles.List(pluginID, action.StoragePrefix)
		if errors.Is(err, pluginfile.ErrInvalidPath) {
			return nil, &runtime.Error{Code: "platform.invalid_request", Message: "storage.file path is invalid"}
		}
		if err != nil {
			return nil, &runtime.Error{Code: "plugin.internal_error", Message: "storage.file list failed", Err: err}
		}
		return map[string]any{
			"root":   action.StorageRoot,
			"prefix": action.StoragePrefix,
			"paths":  paths,
		}, nil
	default:
		return nil, &runtime.Error{
			Code:    "plugin.protocol_violation",
			Message: "received unsupported storage.file operation",
		}
	}
}

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
	if errors.Is(err, pluginhttp.ErrScopeViolation) {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "http.request target is outside the granted scope",
		}
	}
	if errors.Is(err, pluginhttp.ErrInvalidRequest) {
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

func (a *App) pluginCapabilityGranted(ctx context.Context, pluginID, capability string) bool {
	if a == nil || a.pluginLifecycle == nil {
		return false
	}
	for _, granted := range a.pluginLifecycle.grantedCapabilities(ctx, pluginID) {
		if strings.TrimSpace(granted) == capability {
			return true
		}
	}
	return false
}

type grantedScope struct {
	HTTPHosts    []string `json:"http_hosts"`
	StorageRoots []string `json:"storage_roots"`
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
				if len(scope.HTTPHosts) > 0 || len(scope.StorageRoots) > 0 {
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

func currentKVLimits(cfg config.Config) pluginkv.Limits {
	valueLimit := cfg.Storage.KVValueMaxBytes
	if valueLimit <= 0 {
		valueLimit = defaultKVValueMaxBytes
	}
	totalLimitMB := cfg.Storage.KVTotalLimitMB
	if totalLimitMB <= 0 {
		totalLimitMB = defaultKVTotalLimitMegabyte
	}
	return pluginkv.Limits{
		ValueMaxBytes: valueLimit,
		TotalMaxBytes: totalLimitMB * 1024 * 1024,
	}
}

func currentFileLimits(cfg config.Config) pluginfile.Limits {
	fileLimit := cfg.Storage.FileMaxBytes
	if fileLimit <= 0 {
		fileLimit = defaultFileMaxBytes
	}
	totalLimitMB := cfg.Storage.PluginWorkDirMB
	if totalLimitMB <= 0 {
		totalLimitMB = defaultPluginWorkdirMB
	}
	return pluginfile.Limits{
		FileMaxBytes:  fileLimit,
		TotalMaxBytes: totalLimitMB * 1024 * 1024,
	}
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

func (a *App) redactString(value string) string {
	if a == nil || a.redactText == nil {
		return value
	}
	return a.redactText(value)
}

func redactValue(redactText func(string) string, value any) any {
	switch typed := value.(type) {
	case string:
		if redactText == nil {
			return typed
		}
		return redactText(typed)
	case []any:
		result := make([]any, len(typed))
		for index := range typed {
			result[index] = redactValue(redactText, typed[index])
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, inner := range typed {
			result[key] = redactValue(redactText, inner)
		}
		return result
	default:
		return value
	}
}
