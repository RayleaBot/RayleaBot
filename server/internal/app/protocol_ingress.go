package app

import (
	"context"

	"github.com/coder/websocket"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
)

func (s *protocolService) reverseWSIngressAvailable() bool {
	return s != nil && s.adapter != nil
}

func (s *protocolService) reverseWSIngressEnabled() bool {
	return s.transportIngressEnabled(adapter.TransportReverseWS)
}

func (s *protocolService) reverseWSAccessToken() string {
	if s == nil || s.state == nil {
		return ""
	}
	return s.state.Config.OneBot.ReverseWS.AccessToken
}

func (s *protocolService) markReverseWSAuthFailed() {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.MarkReverseWSAuthFailed()
}

func (s *protocolService) attachReverseWS(conn *websocket.Conn) {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.AttachReverseWS(conn)
}

func (s *protocolService) webhookIngressAvailable() bool {
	return s != nil && s.adapter != nil
}

func (s *protocolService) webhookIngressEnabled() bool {
	return s.transportIngressEnabled(adapter.TransportWebhook)
}

func (s *protocolService) webhookAccessToken() string {
	if s == nil || s.state == nil {
		return ""
	}
	return s.state.Config.OneBot.Webhook.AccessToken
}

func (s *protocolService) markWebhookAuthFailed() {
	if s == nil || s.adapter == nil {
		return
	}
	s.adapter.MarkWebhookAuthFailed()
}

func (s *protocolService) acceptWebhookPayload(ctx context.Context, payload []byte) error {
	if s == nil || s.adapter == nil {
		return errProtocolStopped
	}
	return s.adapter.AcceptWebhookPayload(ctx, payload)
}
