package webhook

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
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

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

		replayDecision := s.evaluateReplayProtection(pluginID, route, registration.ReplayProtection, r)
		if replayDecision.reject {
			httpapi.WriteError(w, r, http.StatusUnauthorized, replayDecision.code, "插件 Webhook 重放校验失败", replayDecision.messageKey, map[string]any{
				"plugin_id": pluginID,
				"route":     route,
			})
			return
		}

		if !s.validateWebhookAuth(r.Context(), registration, r.Header.Get(registration.Header), replayDecision.timestampRaw, replayDecision.eventID, body) {
			httpapi.WriteError(w, r, http.StatusUnauthorized, "permission.denied", "当前用户无权执行该操作", "errors.permission.denied", nil)
			return
		}

		// Authentication succeeded: atomically claim the (route, event_id)
		// slot. peek + commitIfAbsent replaces a single observe so a
		// failed-signature request cannot poison the dedup cache, and the
		// commit step refuses concurrent legitimate retries that share the
		// same event_id so replay protection holds under racing callers.
		if replayDecision.dedupKey != "" {
			if !s.dedup.commitIfAbsent(replayDecision.dedupKey, s.now(), replayDecision.dedupTTL) {
				if registration.ReplayProtection.Enforce {
					s.recordReplayMetric("rejected")
					httpapi.WriteError(w, r, http.StatusUnauthorized, "plugin.webhook_replay_rejected", "插件 Webhook 重放校验失败", "errors.plugin.webhook_replay_rejected", map[string]any{
						"plugin_id": pluginID,
						"route":     route,
					})
					return
				}
				s.recordReplayMetric("grace_observed")
			}
		}

		if !s.dispatcher.HasDeliverablePlugin(pluginID) && s.runtime != nil {
			botID := strings.TrimSpace(s.runtime.CurrentBotID())
			if err := s.runtime.EnsurePluginRunning(r.Context(), pluginID, botID); err != nil && s.logger != nil {
				s.logger.Warn(
					"插件 "+pluginID+" Webhook 分发前启动运行时失败，路由："+route,
					"component", "app",
					"plugin_id", pluginID,
					"route", route,
					"err", err.Error(),
				)
			}
		}

		nowTime := s.now()
		eventID := replayDecision.eventID
		if strings.TrimSpace(eventID) == "" {
			eventID = fmt.Sprintf("webhook-%s-%d", route, nowTime.UnixNano())
		}
		webhookMeta := map[string]any{
			"route":       route,
			"received_at": nowTime.Unix(),
		}
		if replayDecision.timestamp > 0 {
			webhookMeta["client_timestamp"] = replayDecision.timestamp
		}
		if strings.TrimSpace(replayDecision.eventID) != "" {
			webhookMeta["client_event_id"] = replayDecision.eventID
		}

		result := s.dispatcher.DispatchToPlugin(r.Context(), pluginID, pluginruntime.Event{
			EventID:        eventID,
			SourceProtocol: "webhook",
			SourceAdapter:  "webhook.gateway",
			EventType:      "webhook.received",
			Timestamp:      nowTime.Unix(),
			Target: &pluginruntime.EventTarget{
				Type: "webhook",
				ID:   route,
				Name: route,
			},
			Actor: &pluginruntime.EventActor{
				ID:   webhookRemoteIP(r.RemoteAddr),
				Role: "remote",
			},
			PayloadFields: map[string]any{"webhook": webhookMeta},
			RawPayload:    s.buildWebhookRawPayload(r, route, body, s.capabilities.CapabilityDeclared(r.Context(), pluginID, "event.raw_payload")),
		})
		if result.Outcome != dispatch.OutcomeDelivered {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
	}
}

func (s *Service) validateWebhookAuth(ctx context.Context, registration Registration, presented, timestampRaw, eventID string, body []byte) bool {
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
		_, _ = sum.Write([]byte(timestampRaw))
		_, _ = sum.Write([]byte("\n"))
		_, _ = sum.Write([]byte(eventID))
		_, _ = sum.Write([]byte("\n"))
		_, _ = sum.Write(body)
		expected := registration.SignaturePrefix + hex.EncodeToString(sum.Sum(nil))
		return hmac.Equal([]byte(strings.TrimSpace(presented)), []byte(expected))
	default:
		return false
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
