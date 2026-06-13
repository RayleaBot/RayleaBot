package adapter

import "context"

func (s *Shell) refreshRuntimeInfo(ctx context.Context, transport TransportKey) {
	if transport == TransportWebhook || s.deps.skipRuntimeInfo {
		return
	}

	lookupCtx, cancel := context.WithTimeout(ctx, defaultIdentityLookupTimeout)
	defer cancel()

	version, versionErr := s.getVersionInfoOnTransport(lookupCtx, transport)
	login, loginErr := s.getLoginInfoOnTransport(lookupCtx, transport)
	if versionErr != nil && loginErr != nil {
		s.clearTransportRuntimeInfo(transport)
		return
	}

	info := TransportRuntimeInfo{
		Provider:        DetectProvider(version.AppName),
		AppName:         version.AppName,
		ProtocolVersion: version.ProtocolVersion,
		AppVersion:      version.AppVersion,
		UserID:          login.ID,
		Nickname:        login.Nickname,
	}
	s.updateTransportRuntimeInfo(transport, info)
}

func (s *Shell) updateTransportRuntimeInfo(transport TransportKey, info TransportRuntimeInfo) {
	s.mu.Lock()
	switch transport {
	case TransportForwardWS:
		s.snapshot.ForwardWS.RuntimeInfo = info
	case TransportReverseWS:
		s.snapshot.ReverseWS.RuntimeInfo = info
	case TransportHTTPAPI:
		s.snapshot.HTTPAPI.RuntimeInfo = info
	default:
		s.mu.Unlock()
		return
	}
	s.refreshAggregateStateLocked()
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) clearTransportRuntimeInfo(transport TransportKey) {
	s.mu.Lock()
	s.clearTransportRuntimeInfoLocked(transport)
	snapshot := cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) clearTransportRuntimeInfoLocked(transport TransportKey) {
	switch transport {
	case TransportForwardWS:
		s.snapshot.ForwardWS.RuntimeInfo = TransportRuntimeInfo{}
	case TransportReverseWS:
		s.snapshot.ReverseWS.RuntimeInfo = TransportRuntimeInfo{}
	case TransportHTTPAPI:
		s.snapshot.HTTPAPI.RuntimeInfo = TransportRuntimeInfo{}
	case TransportWebhook:
		s.snapshot.Webhook.RuntimeInfo = TransportRuntimeInfo{}
	}
}
