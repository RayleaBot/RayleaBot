package pluginwebhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
)

type Registration struct {
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

type Registry struct {
	mu    sync.RWMutex
	items map[string]Registration
}

func NewRegistry() *Registry {
	return &Registry{
		items: make(map[string]Registration),
	}
}

func (r *Registry) Register(item Registration) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[webhookKey(item.PluginID, item.Route)] = item
}

func (r *Registry) Get(pluginID, route string) (Registration, bool) {
	if r == nil {
		return Registration{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[webhookKey(pluginID, route)]
	return item, ok
}

func (r *Registry) DeletePlugin(pluginID string) {
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

func webhookKey(pluginID, route string) string {
	return strings.TrimSpace(pluginID) + "\x00" + strings.TrimSpace(route)
}

type GrantView interface {
	CapabilityGranted(context.Context, string, string) bool
	GrantedWebhookScope(context.Context, string, string) (plugins.WebhookScope, bool)
}

type RuntimeEnsurer interface {
	CurrentBotID() string
	EnsurePluginRunning(context.Context, string, string) error
}

type Deps struct {
	CurrentConfig func() config.Config
	Logger        *slog.Logger
	Registry      *Registry
	Secrets       secrets.Store
	Plugins       *plugins.Catalog
	Dispatcher    *dispatch.Dispatcher
	Runtime       RuntimeEnsurer
	Grants        GrantView
}

type Service struct {
	currentConfig func() config.Config
	logger        *slog.Logger
	registry      *Registry
	secrets       secrets.Store
	plugins       *plugins.Catalog
	dispatcher    *dispatch.Dispatcher
	runtime       RuntimeEnsurer
	grants        GrantView
}

func New(deps Deps) *Service {
	return &Service{
		currentConfig: deps.CurrentConfig,
		logger:        deps.Logger,
		registry:      deps.Registry,
		secrets:       deps.Secrets,
		plugins:       deps.Plugins,
		dispatcher:    deps.Dispatcher,
		runtime:       deps.Runtime,
		grants:        deps.Grants,
	}
}

func (s *Service) RegisterPublicRoutes(router chi.Router) {
	if router == nil {
		return
	}
	router.Post("/api/webhooks/{plugin_id}/{route}", s.HandleWebhook())
}

func (s *Service) Expose(ctx context.Context, pluginID string, action runtime.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "event.expose_webhook") {
		return nil, &runtime.Error{
			Code:    "permission.scope_violation",
			Message: "event.expose_webhook capability is not granted",
		}
	}
	if s.registry == nil {
		return nil, &runtime.Error{
			Code:    "plugin.internal_error",
			Message: "webhook gateway is not available",
		}
	}

	scope, ok := s.grants.GrantedWebhookScope(ctx, pluginID, action.WebhookRoute)
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

	urlValue := s.webhookGatewayURL(pluginID, action.WebhookRoute)
	s.registry.Register(Registration{
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

func (s *Service) HandleWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		route := chi.URLParam(r, "route")

		registration, ok := s.registry.Get(pluginID, route)
		if !ok {
			httpapi.WriteError(w, r, http.StatusNotFound, "platform.resource_missing", "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "webhook",
				"plugin_id":     pluginID,
				"route":         route,
			})
			return
		}
		if !slices.Contains(registration.Methods, r.Method) {
			httpapi.WriteError(w, r, http.StatusNotFound, "platform.resource_missing", "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "webhook",
				"plugin_id":     pluginID,
				"route":         route,
			})
			return
		}

		snapshot, ok := s.plugins.Get(pluginID)
		if !ok || !snapshot.Valid || snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" {
			httpapi.WriteError(w, r, http.StatusNotFound, "platform.resource_missing", "缺少必要资源", "errors.platform.resource_missing", map[string]any{
				"resource_type": "plugin",
				"plugin_id":     pluginID,
			})
			return
		}

		allowed, err := webhookSourceAllowed(r.RemoteAddr, registration.SourceIPs)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}
		if !allowed {
			httpapi.WriteError(w, r, http.StatusForbidden, "permission.denied", "当前用户无权执行该操作", "errors.permission.denied", nil)
			return
		}

		body, err := httpapi.ReadRequestBody(w, r, httpapi.MaxWebhookBodyBytes)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		if !s.validateWebhookAuth(r.Context(), registration, r.Header.Get(registration.Header), body) {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "permission.denied", "当前用户无权执行该操作", "errors.permission.denied", nil)
			return
		}

		if !s.dispatcher.HasDeliverablePlugin(pluginID) && s.runtime != nil {
			botID := strings.TrimSpace(s.runtime.CurrentBotID())
			if err := s.runtime.EnsurePluginRunning(r.Context(), pluginID, botID); err != nil && s.logger != nil {
				s.logger.Warn(
					"ensure runtime before webhook dispatch failed",
					"component", "app",
					"plugin_id", pluginID,
					"err", err.Error(),
				)
			}
		}

		result := s.dispatcher.DispatchToPlugin(r.Context(), pluginID, runtime.Event{
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
			RawPayload: s.buildWebhookRawPayload(r, route, body, s.grants.CapabilityGranted(r.Context(), pluginID, "event.raw_payload")),
		})
		if result.Outcome != dispatch.OutcomeDelivered {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
	}
}

func (s *Service) buildWebhookRawPayload(r *http.Request, route string, body []byte, include bool) any {
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

func (s *Service) validateWebhookAuth(ctx context.Context, registration Registration, presented string, body []byte) bool {
	if s == nil || s.secrets == nil {
		return false
	}
	secretValue, err := s.secrets.Get(ctx, registration.SecretRef)
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

func (s *Service) webhookGatewayURL(pluginID, route string) string {
	cfg := config.Config{}
	if s != nil && s.currentConfig != nil {
		cfg = s.currentConfig()
	}
	host := strings.TrimSpace(cfg.Server.Host)
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	u := &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, fmt.Sprintf("%d", cfg.Server.Port)),
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
