package adapter

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func newTransportSnapshot(cfg config.OneBotConfig) Snapshot {
	forwardURL := strings.TrimSpace(cfg.ForwardWS.URL)
	forwardEnabled := cfg.ForwardWS.Enabled

	snapshot := Snapshot{
		State: StateIdle,
		ForwardWS: TransportSnapshot{
			Enabled:    forwardEnabled,
			Configured: forwardURL != "",
			Endpoint:   sanitizeWSURL(forwardURL),
			State:      TransportStateIdle,
		},
		ReverseWS: TransportSnapshot{
			Enabled:    cfg.ReverseWS.Enabled,
			Configured: strings.TrimSpace(cfg.ReverseWS.URL) != "",
			Endpoint:   sanitizeWSURL(cfg.ReverseWS.URL),
			State:      TransportStateIdle,
		},
		HTTPAPI: TransportSnapshot{
			Enabled:    cfg.HTTPAPI.Enabled,
			Configured: strings.TrimSpace(cfg.HTTPAPI.URL) != "",
			Endpoint:   sanitizeHTTPURL(cfg.HTTPAPI.URL),
			State:      TransportStateIdle,
		},
		Webhook: TransportSnapshot{
			Enabled:    cfg.Webhook.Enabled,
			Configured: strings.TrimSpace(cfg.Webhook.URL) != "",
			Endpoint:   sanitizeHTTPURL(cfg.Webhook.URL),
			State:      TransportStateIdle,
		},
	}
	return snapshot
}

func (s *Shell) markTransportPrimed() {
	s.mu.Lock()
	s.snapshot = newTransportSnapshot(s.cfg)
	s.pendingResponses = make(map[string]chan apiResponse)
	s.recentEventIDs = make(map[string]time.Time)
	s.identityCache = NewIdentityCache(defaultIdentityCacheTTL)
	if s.snapshot.ReverseWS.Enabled && s.snapshot.ReverseWS.Configured {
		s.snapshot.ReverseWS.State = TransportStateListening
	}
	if s.snapshot.Webhook.Enabled && s.snapshot.Webhook.Configured {
		s.snapshot.Webhook.State = TransportStateListening
	}
	if s.snapshot.HTTPAPI.Enabled && s.snapshot.HTTPAPI.Configured {
		s.snapshot.HTTPAPI.State = TransportStateConnected
	}
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
	if s.snapshot.HTTPAPI.Enabled && s.snapshot.HTTPAPI.Configured {
		go s.refreshRuntimeInfo(context.Background(), TransportHTTPAPI)
	}
}

func (s *Shell) forwardWSURL() string {
	return strings.TrimSpace(s.cfg.ForwardWS.URL)
}

func (s *Shell) transportEndpoint(transport TransportKey) string {
	switch transport {
	case TransportForwardWS:
		return sanitizeWSURL(s.forwardWSURL())
	case TransportReverseWS:
		return sanitizeWSURL(s.cfg.ReverseWS.URL)
	case TransportHTTPAPI:
		return sanitizeHTTPURL(s.cfg.HTTPAPI.URL)
	case TransportWebhook:
		return sanitizeHTTPURL(s.cfg.Webhook.URL)
	default:
		return ""
	}
}

func sanitizeHTTPURL(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func (s *Shell) refreshAggregateStateLocked() {
	active := make([]TransportKey, 0, 4)
	if s.snapshot.ForwardWS.State == TransportStateConnecting ||
		s.snapshot.ForwardWS.State == TransportStateConnected ||
		s.snapshot.ForwardWS.State == TransportStateReconnecting ||
		s.snapshot.ForwardWS.State == TransportStateAuthFailed {
		active = append(active, TransportForwardWS)
	}
	if s.snapshot.ReverseWS.State == TransportStateListening || s.snapshot.ReverseWS.State == TransportStateConnected {
		active = append(active, TransportReverseWS)
	}
	if s.snapshot.HTTPAPI.State == TransportStateConnected || s.snapshot.HTTPAPI.State == TransportStateAuthFailed || s.snapshot.HTTPAPI.State == TransportStateReconnecting {
		active = append(active, TransportHTTPAPI)
	}
	if s.snapshot.Webhook.State == TransportStateListening || s.snapshot.Webhook.State == TransportStateConnected {
		active = append(active, TransportWebhook)
	}
	s.snapshot.ActiveTransports = active

	switch {
	case s.snapshot.ForwardWS.State == TransportStateConnected || s.snapshot.ReverseWS.State == TransportStateConnected:
		s.snapshot.State = StateConnected
	case s.snapshot.ForwardWS.State == TransportStateConnecting:
		s.snapshot.State = StateConnecting
	case s.snapshot.ForwardWS.State == TransportStateReconnecting:
		s.snapshot.State = StateReconnecting
	case s.snapshot.ForwardWS.State == TransportStateAuthFailed ||
		s.snapshot.ReverseWS.State == TransportStateAuthFailed ||
		s.snapshot.HTTPAPI.State == TransportStateAuthFailed ||
		s.snapshot.Webhook.State == TransportStateAuthFailed:
		s.snapshot.State = StateAuthFailed
	case s.snapshot.ForwardWS.State == TransportStateStopped ||
		s.snapshot.ReverseWS.State == TransportStateStopped ||
		s.snapshot.HTTPAPI.State == TransportStateStopped ||
		s.snapshot.Webhook.State == TransportStateStopped:
		s.snapshot.State = StateStopped
	default:
		s.snapshot.State = StateIdle
	}
}
