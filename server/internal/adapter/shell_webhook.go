package adapter

import (
	"context"
	"errors"

	"github.com/coder/websocket"
)

func (s *Shell) AcceptWebhookPayload(ctx context.Context, payload []byte) error {
	frame := classifyFrame(websocket.MessageText, payload, s.deps.now())
	if err := s.recordAndValidateFrame(TransportWebhook, frame); err != nil {
		s.markTransportFailure(TransportWebhook, TransportStateListening, errorCodeWebhookInvalidPayload, err)
		return errorf(errorCodeWebhookInvalidPayload, "webhook payload is invalid", err)
	}

	s.mu.Lock()
	s.snapshot.Webhook.State = TransportStateListening
	s.snapshot.Webhook.LastErrorCode = ""
	s.snapshot.Webhook.LastErrorMessage = ""
	s.syncLastErrorLocked()
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)

	s.routeAPIResponse(frame)
	s.forwardSupportedEvent(ctx, TransportWebhook, frame)
	return nil
}

func (s *Shell) MarkReverseWSAuthFailed() {
	s.markTransportFailure(TransportReverseWS, TransportStateAuthFailed, errorCodeReverseWSAuthFailed, errors.New("reverse websocket authentication failed"))
}

func (s *Shell) MarkWebhookAuthFailed() {
	s.markTransportFailure(TransportWebhook, TransportStateAuthFailed, errorCodeWebhookAuthFailed, errors.New("webhook authentication failed"))
}
