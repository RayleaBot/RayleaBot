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

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
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

		replayDecision := s.evaluateReplayProtection(pluginID, route, registration.ReplayProtection, r)
		if replayDecision.reject {
			httpapi.WriteError(w, r, replayDecision.code, map[string]any{
				"plugin_id": pluginID,
				"route":     route,
			})
			return
		}

		if !s.validateWebhookAuth(r.Context(), registration, r.Header.Get(registration.Header), replayDecision.timestampRaw, replayDecision.eventID, body) {
			httpapi.WriteError(w, r, errorcodes.PermissionDenied, nil)
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
					httpapi.WriteError(w, r, errorcodes.PluginWebhookReplayRejected, map[string]any{
						"plugin_id": pluginID,
						"route":     route,
					})
					return
				}
				s.recordReplayMetric("grace_observed")
			}
		}

		if !s.dispatcher.HasDeliverablePlugin(pluginID) {
			if err := s.runtime.EnsurePluginRunning(r.Context(), pluginID); err != nil {
				s.logger.Warn(
					"插件 "+pluginID+" 启动失败，无法处理 Webhook 请求："+err.Error(),
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
		webhookMeta := &chatevent.Webhook{
			Route:      route,
			ReceivedAt: nowTime.Unix(),
		}
		if replayDecision.timestamp > 0 {
			clientTimestamp := replayDecision.timestamp
			webhookMeta.ClientTimestamp = &clientTimestamp
		}
		if strings.TrimSpace(replayDecision.eventID) != "" {
			webhookMeta.ClientEventID = replayDecision.eventID
		}

		_, includeRawPayload := snapshot.Permissions["event.raw_payload"]
		result := s.dispatcher.DispatchToPlugin(r.Context(), pluginID, chatevent.Event{
			EventID:        eventID,
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
			Webhook:    webhookMeta,
			RawPayload: s.buildWebhookRawPayload(r, route, body, includeRawPayload),
		})
		if result.Outcome != dispatch.OutcomeDelivered {
			httpapi.WriteError(w, r, errorcodes.PlatformInternalError, nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, map[string]any{"accepted": true})
	}
}

func (s *Service) validateWebhookAuth(ctx context.Context, registration Registration, presented, timestampRaw, eventID string, body []byte) bool {
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
