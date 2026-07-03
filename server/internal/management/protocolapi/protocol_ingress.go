package protocolapi

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/coder/websocket"
)

func (s *ProtocolService) reverseWSIngressAvailable() bool {
	return s != nil && s.adapter != nil
}

func (s *ProtocolService) reverseWSIngressEnabled() bool {
	return s.transportIngressEnabled(onebot11.TransportReverseWS)
}

func (s *ProtocolService) reverseWSAccessToken() string {
	if s == nil || s.config == nil {
		return ""
	}
	return s.config.CurrentConfig().OneBot.ReverseWS.AccessToken
}

func (s *ProtocolService) reverseWSAccessTokenQueryCompat() bool {
	if s == nil || s.config == nil {
		return false
	}
	return s.config.CurrentConfig().OneBot.ReverseWS.AccessTokenQueryCompat
}

func (s *ProtocolService) markReverseWSAuthFailed() {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.MarkReverseWSAuthFailed()
}

func (s *ProtocolService) attachReverseWS(conn *websocket.Conn) {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.AttachReverseWS(conn)
}

func (s *ProtocolService) webhookIngressAvailable() bool {
	return s != nil && s.adapter != nil
}

func (s *ProtocolService) webhookIngressEnabled() bool {
	return s.transportIngressEnabled(onebot11.TransportWebhook)
}

func (s *ProtocolService) webhookAccessToken() string {
	if s == nil || s.config == nil {
		return ""
	}
	return s.config.CurrentConfig().OneBot.Webhook.AccessToken
}

func (s *ProtocolService) webhookAccessTokenQueryCompat() bool {
	if s == nil || s.config == nil {
		return false
	}
	return s.config.CurrentConfig().OneBot.Webhook.AccessTokenQueryCompat
}

func (s *ProtocolService) markWebhookAuthFailed() {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.MarkWebhookAuthFailed()
}

func (s *ProtocolService) acceptWebhookPayload(ctx context.Context, payload []byte) error {
	if s == nil || s.adapter == nil {
		return configruntime.ErrProtocolStopped
	}
	return s.adapter.AcceptWebhookPayload(ctx, payload)
}
