package webhook

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/httpapi"
	"github.com/go-chi/chi/v5"
)

func (s *Service) HandleWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pluginID := chi.URLParam(r, "plugin_id")
		route := chi.URLParam(r, "route")

		registration, ok := s.registry.Get(pluginID, route)
		if !ok {
			httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, map[string]any{
				"resource_type": "webhook",
				"plugin_id":     pluginID,
				"route":         route,
			})
			return
		}
		if !slices.Contains(registration.Methods, r.Method) {
			httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, map[string]any{
				"resource_type": "webhook",
				"plugin_id":     pluginID,
				"route":         route,
			})
			return
		}

		snapshot, ok := s.plugins.Get(pluginID)
		if !ok || !snapshot.Valid || snapshot.RegistrationState != "installed" || snapshot.DesiredState != "enabled" {
			httpapi.WriteError(w, r, errorcodes.PlatformResourceNotFound, map[string]any{
				"resource_type": "plugin",
				"plugin_id":     pluginID,
			})
			return
		}

		allowed, err := webhookSourceAllowed(r.RemoteAddr, registration.SourceIPs)
		if err != nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}
		if !allowed {
			httpapi.WriteError(w, r, errorcodes.PermissionDenied, nil)
			return
		}

		maxBodyBytes := httpapi.MaxWebhookBodyBytes
		if registration.MaxBodyBytes > 0 && int64(registration.MaxBodyBytes) < maxBodyBytes {
			maxBodyBytes = int64(registration.MaxBodyBytes)
		}
		body, err := httpapi.ReadRequestBody(w, r, maxBodyBytes)
		if err != nil {
			httpapi.WriteError(w, r, errorcodes.PlatformInvalidRequest, nil)
			return
		}

		if !s.dispatcher.HasDeliverablePlugin(pluginID) {
			if err := s.runtime.EnsurePluginRunning(r.Context(), pluginID); err != nil {
				s.logger.Warn(
					"插件启动失败，无法处理 Webhook 请求",
					"component", "app",
					"plugin_id", pluginID,
					"route", route,
					"err", err.Error(),
				)
			}
		}

		nowTime := s.now()
		result := s.dispatcher.DispatchToPlugin(r.Context(), pluginID, chatevent.Event{
			EventID:        fmt.Sprintf("webhook-%s-%d", route, nowTime.UnixNano()),
			SourceProtocol: "webhook",
			SourceAdapter:  "webhook.gateway",
			EventType:      "webhook.received",
			Timestamp:      nowTime.Unix(),
			Target: &chatevent.Target{
				Type: "webhook",
				ID:   route,
				Name: route,
			},
			Actor: &chatevent.Actor{
				ID:   webhookRemoteIP(r.RemoteAddr),
				Role: "remote",
			},
			Webhook:    &chatevent.Webhook{Route: route, ReceivedAt: nowTime.Unix()},
			RawPayload: buildWebhookRawPayload(r, route, body),
		})
		if result.Outcome != dispatch.OutcomeDelivered {
			httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
	}
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

// buildWebhookRawPayload keeps the exact request body so plugins can verify
// signatures over the bytes the caller signed.
func buildWebhookRawPayload(r *http.Request, route string, body []byte) any {
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

	if utf8.Valid(body) {
		payload["body_text"] = string(body)
		return payload
	}
	payload["body_base64"] = base64.StdEncoding.EncodeToString(body)
	return payload
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
