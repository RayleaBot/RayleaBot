package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
)

type pluginWebhookRegistration struct {
	PluginID        string
	Route           string
	Methods         []string
	AuthStrategy    string
	Header          string
	SecretRef       string
	SignaturePrefix string
	SourceIPs       []string
	URL             string
}

type pluginWebhookRegistry struct {
	mu    sync.RWMutex
	items map[string]pluginWebhookRegistration
}

func newPluginWebhookRegistry() *pluginWebhookRegistry {
	return &pluginWebhookRegistry{
		items: make(map[string]pluginWebhookRegistration),
	}
}

func (r *pluginWebhookRegistry) Register(item pluginWebhookRegistration) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[pluginWebhookKey(item.PluginID, item.Route)] = item
}

func (r *pluginWebhookRegistry) Get(pluginID, route string) (pluginWebhookRegistration, bool) {
	if r == nil {
		return pluginWebhookRegistration{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[pluginWebhookKey(pluginID, route)]
	return item, ok
}

func (r *pluginWebhookRegistry) DeletePlugin(pluginID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	prefix := pluginID + "\x00"
	for key := range r.items {
		if strings.HasPrefix(key, prefix) {
			delete(r.items, key)
		}
	}
}

func pluginWebhookKey(pluginID, route string) string {
	return strings.TrimSpace(pluginID) + "\x00" + strings.TrimSpace(route)
}

func (a *App) executeExposeWebhook(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if !a.pluginCapabilityGranted(ctx, pluginID, "event.expose_webhook") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "event.expose_webhook capability is not granted",
		}
	}
	if a == nil || a.webhooks == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "webhook gateway is not available",
		}
	}

	scope, ok := a.grantedWebhookScope(ctx, pluginID, action.WebhookRoute)
	if !ok {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "event.expose_webhook route is outside the granted scope",
		}
	}
	if strings.TrimSpace(scope.AuthStrategy) != strings.TrimSpace(action.WebhookAuthStrategy) ||
		strings.TrimSpace(scope.Header) != strings.TrimSpace(action.WebhookHeader) ||
		strings.TrimSpace(scope.SecretRef) != strings.TrimSpace(action.WebhookSecretRef) {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "event.expose_webhook settings exceed the granted scope",
		}
	}

	sourceIPs := selectWebhookSourceIPs(scope.SourceIPs, action.WebhookSourceIPs)
	if !webhookSourceIPsWithinScope(scope.SourceIPs, sourceIPs) {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "event.expose_webhook source_ips exceed the granted scope",
		}
	}

	urlValue := a.webhookGatewayURL(pluginID, action.WebhookRoute)
	a.webhooks.Register(pluginWebhookRegistration{
		PluginID:        pluginID,
		Route:           action.WebhookRoute,
		Methods:         append([]string(nil), action.WebhookMethods...),
		AuthStrategy:    action.WebhookAuthStrategy,
		Header:          action.WebhookHeader,
		SecretRef:       action.WebhookSecretRef,
		SignaturePrefix: action.WebhookSignaturePrefix,
		SourceIPs:       sourceIPs,
		URL:             urlValue,
	})
	return map[string]any{
		"route": action.WebhookRoute,
		"url":   urlValue,
	}, nil
}

func (a *App) handlePluginWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		route := chi.URLParam(r, "route")

		registration, ok := a.webhooks.Get(pluginID, route)
		if !ok {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "webhook",
				"plugin_id":     pluginID,
				"route":         route,
			})
			return
		}
		if !slices.Contains(registration.Methods, r.Method) {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "webhook",
				"plugin_id":     pluginID,
				"route":         route,
			})
			return
		}

		snapshot, ok := a.Plugins.Get(pluginID)
		if !ok || !snapshot.Valid || snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" {
			writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "plugin",
				"plugin_id":     pluginID,
			})
			return
		}

		allowed, err := webhookSourceAllowed(r.RemoteAddr, registration.SourceIPs)
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		if !allowed {
			writeAppError(w, r, http.StatusForbidden, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied", nil)
			return
		}

		body, err := readRequestBody(w, r, maxWebhookBodyBytes)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if !a.validateWebhookAuth(r.Context(), registration, r.Header.Get(registration.Header), body) {
			writeAppError(w, r, http.StatusUnauthorized, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied", nil)
			return
		}

		if !a.Dispatcher.HasPlugin(pluginID) && a.pluginLifecycle != nil {
			if botID := a.pluginLifecycle.currentBotID(); botID != "" {
				if err := a.pluginLifecycle.ensurePluginRunning(r.Context(), pluginID, botID); err != nil {
					a.Logger.Warn(
						"ensure runtime before webhook dispatch failed",
						"component", "app",
						"plugin_id", pluginID,
						"err", err.Error(),
					)
				}
			}
		}

		result := a.Dispatcher.DispatchToPlugin(r.Context(), pluginID, runtime.Event{
			EventID:        fmt.Sprintf("webhook-%s-%d", route, time.Now().UnixNano()),
			SourceProtocol: "webhook",
			SourceAdapter:  "webhook.gateway",
			EventType:      "webhook.received",
			Timestamp:      time.Now().Unix(),
			Target: &runtime.EventTarget{
				Type: "webhook",
				ID:   route,
				Name: route,
			},
			Actor: &runtime.EventActor{
				ID:   webhookRemoteIP(r.RemoteAddr),
				Role: "remote",
			},
			RawPayload: a.buildWebhookRawPayload(r, route, body, a.pluginCapabilityGranted(r.Context(), pluginID, "event.raw_payload")),
		})
		if result.Outcome != dispatch.OutcomeDelivered {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
	}
}

func (a *App) buildWebhookRawPayload(r *http.Request, route string, body []byte, include bool) any {
	if !include {
		return nil
	}

	payload := map[string]any{
		"route":        route,
		"method":       r.Method,
		"content_type": r.Header.Get("Content-Type"),
		"headers":      cloneWebhookHeaders(r.Header),
		"query":        cloneWebhookQuery(r.URL.Query()),
	}
	if len(body) == 0 {
		return payload
	}

	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.Contains(contentType, "application/json") {
		var decoded any
		if err := json.Unmarshal(body, &decoded); err == nil {
			payload["body_json"] = decoded
			return payload
		}
	}
	if utf8.Valid(body) {
		payload["body_text"] = string(body)
		return payload
	}
	payload["body_base64"] = base64.StdEncoding.EncodeToString(body)
	return payload
}

func (a *App) validateWebhookAuth(ctx context.Context, registration pluginWebhookRegistration, presented string, body []byte) bool {
	if a == nil || a.Secrets == nil {
		return false
	}
	secretValue, err := a.Secrets.Get(ctx, registration.SecretRef)
	if err != nil {
		return false
	}

	switch registration.AuthStrategy {
	case "fixed_token":
		return hmac.Equal([]byte(strings.TrimSpace(presented)), secretValue)
	case "hmac_sha256":
		sum := hmac.New(sha256.New, secretValue)
		_, _ = sum.Write(body)
		expected := registration.SignaturePrefix + hex.EncodeToString(sum.Sum(nil))
		return hmac.Equal([]byte(strings.TrimSpace(presented)), []byte(expected))
	default:
		return false
	}
}

func (a *App) grantedWebhookScope(ctx context.Context, pluginID, route string) (plugins.WebhookScope, bool) {
	scope := a.pluginGrantedScope(ctx, pluginID, "event.expose_webhook")
	route = strings.TrimSpace(route)
	for _, item := range scope.Webhooks {
		if strings.TrimSpace(item.Route) == route {
			return item, true
		}
	}
	return plugins.WebhookScope{}, false
}

func (a *App) webhookGatewayURL(pluginID, route string) string {
	host := strings.TrimSpace(a.Config.Server.Host)
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	u := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, fmt.Sprintf("%d", a.Config.Server.Port)),
		Path:   fmt.Sprintf("/api/webhooks/%s/%s", pluginID, route),
	}
	return u.String()
}

func webhookSourceAllowed(remoteAddr string, allowed []string) (bool, error) {
	if len(allowed) == 0 {
		return true, nil
	}
	remoteIP := net.ParseIP(webhookRemoteIP(remoteAddr))
	if remoteIP == nil {
		return false, nil
	}
	for _, candidate := range allowed {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if strings.Contains(candidate, "/") {
			_, network, err := net.ParseCIDR(candidate)
			if err != nil {
				return false, err
			}
			if network.Contains(remoteIP) {
				return true, nil
			}
			continue
		}
		allowedIP := net.ParseIP(candidate)
		if allowedIP != nil && allowedIP.Equal(remoteIP) {
			return true, nil
		}
	}
	return false, nil
}

func webhookRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func cloneWebhookHeaders(headers http.Header) map[string]any {
	result := make(map[string]any, len(headers))
	for key, values := range headers {
		copied := make([]string, len(values))
		copy(copied, values)
		result[key] = copied
	}
	return result
}

func cloneWebhookQuery(values url.Values) map[string]any {
	result := make(map[string]any, len(values))
	for key, items := range values {
		copied := make([]string, len(items))
		copy(copied, items)
		result[key] = copied
	}
	return result
}

func selectWebhookSourceIPs(scopeValues []string, actionValues []string) []string {
	if len(actionValues) == 0 {
		return append([]string(nil), scopeValues...)
	}
	return append([]string(nil), actionValues...)
}

func webhookSourceIPsWithinScope(scopeValues []string, actionValues []string) bool {
	if len(scopeValues) == 0 {
		return true
	}
	if len(actionValues) == 0 {
		return true
	}
	allowed := make(map[string]struct{}, len(scopeValues))
	for _, value := range scopeValues {
		allowed[strings.TrimSpace(value)] = struct{}{}
	}
	for _, value := range actionValues {
		if _, ok := allowed[strings.TrimSpace(value)]; !ok {
			return false
		}
	}
	return true
}
