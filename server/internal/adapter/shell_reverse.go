package adapter

import (
	"context"

	"github.com/coder/websocket"
)

func (s *Shell) AttachReverseWS(conn *websocket.Conn) {
	if conn == nil {
		return
	}

	done := make(chan struct{})
	var previous *websocket.Conn

	s.mu.Lock()
	if s.stopping || !s.started {
		s.mu.Unlock()
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return
	}
	if s.reverseConn != nil {
		previous = s.reverseConn
	}
	s.reverseConn = conn
	s.reverseDone = done
	s.mu.Unlock()

	if previous != nil {
		_ = previous.Close(websocket.StatusNormalClosure, "")
	}

	go s.handleReverseSession(conn, done)
}

func (s *Shell) handleReverseSession(conn *websocket.Conn, done chan struct{}) {
	ctx := context.Background()
	defer func() {
		defer close(done)
		_ = conn.Close(websocket.StatusNormalClosure, "")
		s.mu.Lock()
		current := s.reverseConn == conn
		if current {
			s.reverseConn = nil
			if s.reverseDone == done {
				s.reverseDone = nil
			}
		}
		if !current && !s.stopping {
			s.mu.Unlock()
			return
		}
		s.clearTransportRuntimeInfoLocked(TransportReverseWS)
		if s.stopping && s.snapshot.ReverseWS.Enabled && s.snapshot.ReverseWS.Configured {
			s.snapshot.ReverseWS.State = TransportStateStopped
		} else if s.snapshot.ReverseWS.Enabled && s.snapshot.ReverseWS.Configured {
			s.snapshot.ReverseWS.State = TransportStateListening
		} else {
			s.snapshot.ReverseWS.State = TransportStateIdle
		}
		s.refreshAggregateStateLocked()
		snapshot := cloneSnapshot(s.snapshot)
		handler := s.stateHandler
		s.mu.Unlock()
		s.emitStateSnapshot(handler, snapshot)
	}()

	ready, err := s.waitForReadyFrame(ctx, TransportReverseWS, conn)
	if err != nil {
		if ctx.Err() != nil || s.isStopping() {
			return
		}
		s.markTransportFailure(TransportReverseWS, TransportStateListening, errorCodeConnectionLost, err)
		return
	}

	s.mu.Lock()
	s.snapshot.ReverseWS.State = TransportStateConnected
	s.snapshot.ReverseWS.LastErrorCode = ""
	s.snapshot.ReverseWS.LastErrorMessage = ""
	s.snapshot.ReadyFrameSeen = true
	s.snapshot.ConnectedAt = cloneTime(&ready.ObservedAt)
	s.syncLastErrorLocked()
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
	go s.refreshRuntimeInfo(ctx, TransportReverseWS)

	if readyHandler := s.currentReadyHandler(); readyHandler != nil {
		go readyHandler(ctx)
	}

	if err := s.readLoop(ctx, TransportReverseWS, conn); err != nil && ctx.Err() == nil && !s.isStopping() {
		s.markTransportFailure(TransportReverseWS, TransportStateListening, errorCodeConnectionLost, err)
	}
}

func (s *Shell) isStopping() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopping
}
