package action

import (
	"encoding/json"
	"strings"

	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
)

func parseEventExposeWebhookAction(raw json.RawMessage) (*Action, error) {
	var frame runtimeprotocol.ProtocolActionEventExposeWebhookFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil, errorf(codePluginProtocolViolation, "plugin returned malformed event.expose_webhook data", err)
	}

	route := strings.TrimSpace(frame.Route)
	authStrategy := strings.TrimSpace(frame.AuthStrategy)
	header := strings.TrimSpace(frame.Header)
	secretRef := strings.TrimSpace(frame.SecretRef)
	if route == "" || authStrategy == "" || header == "" || secretRef == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required event.expose_webhook fields", nil)
	}
	if len(frame.Methods) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required event.expose_webhook fields", nil)
	}

	methods := make([]string, 0, len(frame.Methods))
	seenMethods := make(map[string]struct{}, len(frame.Methods))
	for _, method := range frame.Methods {
		method = strings.ToUpper(strings.TrimSpace(method))
		if method != "POST" {
			return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported event.expose_webhook method", nil)
		}
		if _, ok := seenMethods[method]; ok {
			continue
		}
		seenMethods[method] = struct{}{}
		methods = append(methods, method)
	}
	if len(methods) == 0 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required event.expose_webhook fields", nil)
	}

	switch authStrategy {
	case "fixed_token", "hmac_sha256":
	default:
		return nil, errorf(codePluginProtocolViolation, "plugin action frame uses unsupported event.expose_webhook auth_strategy", nil)
	}
	signaturePrefix := strings.TrimSpace(frame.SignaturePrefix)
	if authStrategy == "hmac_sha256" && signaturePrefix == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required event.expose_webhook signature_prefix", nil)
	}

	sourceIPs := make([]string, 0, len(frame.SourceIPs))
	seenSources := make(map[string]struct{}, len(frame.SourceIPs))
	for _, value := range frame.SourceIPs {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seenSources[value]; ok {
			continue
		}
		seenSources[value] = struct{}{}
		sourceIPs = append(sourceIPs, value)
	}

	if frame.ReplayProtection == nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required event.expose_webhook replay_protection", nil)
	}
	timestampHeader := strings.TrimSpace(frame.ReplayProtection.TimestampHeader)
	eventIDHeader := strings.TrimSpace(frame.ReplayProtection.EventIDHeader)
	if timestampHeader == "" || eventIDHeader == "" {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame is missing required event.expose_webhook replay_protection headers", nil)
	}
	if frame.ReplayProtection.ToleranceSeconds < 1 || frame.ReplayProtection.ToleranceSeconds > 3600 {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame replay_protection.tolerance_seconds is out of range", nil)
	}
	if frame.ReplayProtection.Enforce == nil {
		return nil, errorf(codePluginProtocolViolation, "plugin action frame replay_protection.enforce must be a boolean", nil)
	}
	replay := &WebhookReplayProtection{
		TimestampHeader:  timestampHeader,
		EventIDHeader:    eventIDHeader,
		ToleranceSeconds: frame.ReplayProtection.ToleranceSeconds,
		Enforce:          *frame.ReplayProtection.Enforce,
	}

	return &Action{
		Kind:                    "event.expose_webhook",
		WebhookRoute:            route,
		WebhookMethods:          methods,
		WebhookAuthStrategy:     authStrategy,
		WebhookHeader:           header,
		WebhookSecretRef:        secretRef,
		WebhookSignaturePrefix:  signaturePrefix,
		WebhookSourceIPs:        sourceIPs,
		WebhookReplayProtection: replay,
	}, nil
}
