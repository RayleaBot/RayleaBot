package webhook

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
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
					"ensure runtime before webhook dispatch failed",
					"component", "app",
					"plugin_id", pluginID,
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

		result := s.dispatcher.DispatchToPlugin(r.Context(), pluginID, runtimeprotocol.Event{
			EventID:        eventID,
			SourceProtocol: "webhook",
			SourceAdapter:  "webhook.gateway",
			EventType:      "webhook.received",
			Timestamp:      nowTime.Unix(),
			Target: &runtimeprotocol.EventTarget{
				Type: "webhook",
				ID:   route,
				Name: route,
			},
			Actor: &runtimeprotocol.EventActor{
				ID:   webhookRemoteIP(r.RemoteAddr),
				Role: "remote",
			},
			PayloadFields: map[string]any{"webhook": webhookMeta},
			RawPayload:    s.buildWebhookRawPayload(r, route, body, s.grants.CapabilityGranted(r.Context(), pluginID, "event.raw_payload")),
		})
		if result.Outcome != dispatch.OutcomeDelivered {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
	}
}
