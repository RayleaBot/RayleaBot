package onebot11

import (
	"net/http"
	"time"
)

func newHTTPAPIClient(timeout time.Duration, base http.RoundTripper) (*http.Client, *http.Transport) {
	// Only cloned transports belong to this shell; custom round trippers retain
	// their caller-managed lifetime and configuration.
	var owned *http.Transport
	if transport, ok := base.(*http.Transport); ok {
		owned = transport.Clone()
		owned.MaxIdleConnsPerHost = 16
		owned.MaxIdleConns = 32
		base = owned
	}
	return &http.Client{Transport: base, Timeout: timeout}, owned
}

// A request may enter Do after its client was retired, reopening that client's
// idle pool. Run after closing the response body so the last late request also
// releases its connections, without interrupting other active requests.
func (s *Shell) closeRetiredHTTPIdle(client *http.Client, owned *http.Transport) {
	if owned == nil {
		return
	}
	s.mu.RLock()
	retired := s.httpClient != client || s.stopping
	s.mu.RUnlock()
	if retired {
		owned.CloseIdleConnections()
	}
}

// A retired client's requests still complete for their callers, but their
// results must not change the status of a reconfigured or stopped adapter.
func (s *Shell) markHTTPAPIFailure(client *http.Client, state TransportState, code string, err error) {
	s.mu.Lock()
	if s.httpClient != client || s.stopping {
		s.mu.Unlock()
		return
	}
	s.markTransportFailureLocked(TransportHTTPAPI, state, code, err)
	snapshot, handler := cloneSnapshot(s.snapshot), s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}

func (s *Shell) markHTTPAPISuccess(client *http.Client) {
	s.mu.Lock()
	if s.httpClient != client || s.stopping {
		s.mu.Unlock()
		return
	}
	s.snapshot.HTTPAPI.State = TransportStateConnected
	s.snapshot.HTTPAPI.LastErrorCode = ""
	s.snapshot.HTTPAPI.LastErrorMessage = ""
	s.syncLastErrorLocked()
	s.refreshAggregateStateLocked()
	snapshot, handler := cloneSnapshot(s.snapshot), s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)
}
