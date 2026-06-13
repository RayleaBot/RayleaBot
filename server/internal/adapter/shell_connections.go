package adapter

import "github.com/coder/websocket"

func (s *Shell) currentWSConn() (*websocket.Conn, TransportKey, Snapshot) {
	return s.currentWSConnForTransport("")
}

func (s *Shell) currentWSConnForTransport(transport TransportKey) (*websocket.Conn, TransportKey, Snapshot) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := cloneSnapshot(s.snapshot)
	switch transport {
	case TransportForwardWS:
		if s.conn != nil && snapshot.ForwardWS.State == TransportStateConnected {
			return s.conn, TransportForwardWS, snapshot
		}
		return nil, "", snapshot
	case TransportReverseWS:
		if s.reverseConn != nil && snapshot.ReverseWS.State == TransportStateConnected {
			return s.reverseConn, TransportReverseWS, snapshot
		}
		return nil, "", snapshot
	case TransportHTTPAPI:
		return nil, "", snapshot
	}

	switch {
	case s.conn != nil && snapshot.ForwardWS.State == TransportStateConnected:
		return s.conn, TransportForwardWS, snapshot
	case s.reverseConn != nil && snapshot.ReverseWS.State == TransportStateConnected:
		return s.reverseConn, TransportReverseWS, snapshot
	default:
		return nil, "", snapshot
	}
}
