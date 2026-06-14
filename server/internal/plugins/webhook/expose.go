package webhook

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func (s *Service) Expose(ctx context.Context, pluginID string, action runtimeaction.Action) (map[string]any, error) {
	if s == nil || s.grants == nil || !s.grants.CapabilityGranted(ctx, pluginID, "event.expose_webhook") {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "event.expose_webhook capability is not granted",
		}
	}
	if s.registry == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.internal_error",
			Message: "webhook gateway is not available",
		}
	}
	if action.WebhookReplayProtection == nil {
		return nil, &runtimemanager.Error{
			Code:    "plugin.protocol_violation",
			Message: "event.expose_webhook requires replay_protection",
		}
	}

	scope, ok := s.grants.GrantedWebhookScope(ctx, pluginID, action.WebhookRoute)
	if !ok {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "event.expose_webhook route is outside the granted scope",
		}
	}
	if strings.TrimSpace(scope.AuthStrategy) != strings.TrimSpace(action.WebhookAuthStrategy) ||
		strings.TrimSpace(scope.Header) != strings.TrimSpace(action.WebhookHeader) ||
		strings.TrimSpace(scope.SecretRef) != strings.TrimSpace(action.WebhookSecretRef) {
		return nil, &runtimemanager.Error{
			Code:    "permission.scope_violation",
			Message: "event.expose_webhook settings exceed the granted scope",
		}
	}

	sourceIPs := selectWebhookSourceIPs(scope.SourceIPs, action.WebhookSourceIPs)
	if !webhookSourceIPsWithinScope(scope.SourceIPs, sourceIPs) {
		return nil, &runtimemanager.Error{
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
		ReplayProtection: ReplayProtection{
			TimestampHeader:  action.WebhookReplayProtection.TimestampHeader,
			EventIDHeader:    action.WebhookReplayProtection.EventIDHeader,
			ToleranceSeconds: action.WebhookReplayProtection.ToleranceSeconds,
			Enforce:          action.WebhookReplayProtection.Enforce,
		},
	})
	return map[string]any{
		"route": action.WebhookRoute,
		"url":   urlValue,
	}, nil
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
