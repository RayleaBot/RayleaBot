package adapter

import "strings"

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
