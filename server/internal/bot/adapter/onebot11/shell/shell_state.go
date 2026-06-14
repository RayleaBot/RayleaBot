package shell

import (
	"strings"
	"time"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"github.com/coder/websocket"
)

func (s *Shell) recordFrame(frame adapterintake.ClassifiedFrame) Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	applyFrameSummary(&s.snapshot, frame)
	return cloneSnapshot(s.snapshot)
}
func (s *Shell) setConn(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conn = conn
}
func (s *Shell) clearConn(target *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if target == nil || s.conn == target {
		s.conn = nil
	}
}
func (s *Shell) clearReverseConn(target *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if target == nil || s.reverseConn == target {
		s.reverseConn = nil
	}
}
func (s *Shell) markConnecting() {
	s.mu.Lock()
	s.snapshot.ForwardWS.State = TransportStateConnecting
	s.snapshot.ForwardWS.LastErrorCode = ""
	s.snapshot.ForwardWS.LastErrorMessage = ""
	s.snapshot.ForwardWS.RuntimeInfo = TransportRuntimeInfo{}
	s.snapshot.LastErrorCode = ""
	s.snapshot.LastErrorMessage = ""
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
	s.snapshot.LastFrameCategory = ""
	s.snapshot.LastFrameType = ""
	s.snapshot.LastFrameAt = nil
	s.snapshot.HeartbeatSeen = false
	s.snapshot.LastHeartbeatAt = nil
	s.snapshot.HeartbeatInterval = 0
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}
func (s *Shell) markConnected(now time.Time) {
	s.mu.Lock()
	s.snapshot.ForwardWS.State = TransportStateConnected
	s.snapshot.ForwardWS.LastErrorCode = ""
	s.snapshot.ForwardWS.LastErrorMessage = ""
	s.snapshot.LastErrorCode = ""
	s.snapshot.LastErrorMessage = ""
	s.snapshot.ReadyFrameSeen = true
	s.snapshot.ConnectedAt = cloneTime(&now)
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}
func (s *Shell) markAuthFailed(err error) {
	s.mu.Lock()
	s.snapshot.ForwardWS.State = TransportStateAuthFailed
	s.snapshot.ForwardWS.LastErrorCode = errorCodeForwardWSConnectFail
	s.snapshot.ForwardWS.LastErrorMessage = summarizeError(err)
	s.snapshot.ForwardWS.RuntimeInfo = TransportRuntimeInfo{}
	s.snapshot.LastErrorCode = errorCodeForwardWSConnectFail
	s.snapshot.LastErrorMessage = summarizeError(err)
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}
func (s *Shell) markReconnecting(code string, err error) {
	s.mu.Lock()
	s.snapshot.ForwardWS.State = TransportStateReconnecting
	s.snapshot.ForwardWS.LastErrorCode = code
	s.snapshot.ForwardWS.LastErrorMessage = summarizeError(err)
	s.snapshot.ForwardWS.RuntimeInfo = TransportRuntimeInfo{}
	s.snapshot.LastErrorCode = code
	s.snapshot.LastErrorMessage = summarizeError(err)
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}
func (s *Shell) markStopped() {
	s.mu.Lock()
	s.snapshot.ForwardWS.State = TransportStateStopped
	s.snapshot.ForwardWS.RuntimeInfo = TransportRuntimeInfo{}
	if s.snapshot.ReverseWS.Configured && s.snapshot.ReverseWS.Enabled {
		s.snapshot.ReverseWS.State = TransportStateStopped
	} else {
		s.snapshot.ReverseWS.State = TransportStateIdle
	}
	s.snapshot.ReverseWS.RuntimeInfo = TransportRuntimeInfo{}
	if s.snapshot.Webhook.Configured && s.snapshot.Webhook.Enabled {
		s.snapshot.Webhook.State = TransportStateStopped
	} else {
		s.snapshot.Webhook.State = TransportStateIdle
	}
	s.snapshot.Webhook.RuntimeInfo = TransportRuntimeInfo{}
	if s.snapshot.HTTPAPI.Configured && s.snapshot.HTTPAPI.Enabled {
		s.snapshot.HTTPAPI.State = TransportStateStopped
	} else {
		s.snapshot.HTTPAPI.State = TransportStateIdle
	}
	s.snapshot.HTTPAPI.RuntimeInfo = TransportRuntimeInfo{}
	s.snapshot.ReadyFrameSeen = false
	s.snapshot.ConnectedAt = nil
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}
func (s *Shell) emitStateSnapshot(handler func(Snapshot), snapshot Snapshot) {
	if handler == nil {
		return
	}
	handler(snapshot)
}

func (s *Shell) markTransportFailure(transport TransportKey, fallback TransportState, code string, err error) {
	s.mu.Lock()
	switch transport {
	case TransportReverseWS:
		s.snapshot.ReverseWS.State = fallback
		s.snapshot.ReverseWS.LastErrorCode = code
		s.snapshot.ReverseWS.LastErrorMessage = summarizeError(err)
	case TransportHTTPAPI:
		s.snapshot.HTTPAPI.State = fallback
		s.snapshot.HTTPAPI.LastErrorCode = code
		s.snapshot.HTTPAPI.LastErrorMessage = summarizeError(err)
	case TransportWebhook:
		s.snapshot.Webhook.State = fallback
		s.snapshot.Webhook.LastErrorCode = code
		s.snapshot.Webhook.LastErrorMessage = summarizeError(err)
	case TransportForwardWS:
		s.snapshot.ForwardWS.State = fallback
		s.snapshot.ForwardWS.LastErrorCode = code
		s.snapshot.ForwardWS.LastErrorMessage = summarizeError(err)
	}
	s.clearTransportRuntimeInfoLocked(transport)
	s.snapshot.LastErrorCode = code
	s.snapshot.LastErrorMessage = summarizeError(err)
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) syncLastErrorLocked() {
	for _, transport := range []TransportSnapshot{
		s.snapshot.ForwardWS,
		s.snapshot.ReverseWS,
		s.snapshot.HTTPAPI,
		s.snapshot.Webhook,
	} {
		if strings.TrimSpace(transport.LastErrorCode) == "" {
			continue
		}
		s.snapshot.LastErrorCode = transport.LastErrorCode
		s.snapshot.LastErrorMessage = transport.LastErrorMessage
		return
	}
	s.snapshot.LastErrorCode = ""
	s.snapshot.LastErrorMessage = ""
}
