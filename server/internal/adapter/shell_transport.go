package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"

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

func (s *Shell) isDuplicateEvent(eventID string, observedAt time.Time) bool {
	if strings.TrimSpace(eventID) == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := observedAt.Add(-recentEventDedupRetention)
	for key, seenAt := range s.recentEventIDs {
		if seenAt.Before(cutoff) {
			delete(s.recentEventIDs, key)
		}
	}
	if _, ok := s.recentEventIDs[eventID]; ok {
		s.dedupDrops++
		if s.metrics != nil {
			s.metrics.IncAdapterDedupDrop()
			s.metrics.IncEventPipelineStage("adapter", "dedup_drop")
		}
		return true
	}
	s.recentEventIDs[eventID] = observedAt
	if s.metrics != nil {
		s.metrics.IncEventPipelineStage("adapter", "accepted")
	}
	return false
}

// DedupDropsSnapshot returns the cumulative number of inbound events dropped
// because their event id matched a recently observed event within the
// dedup retention window. The counter is monotonically non-decreasing and
// safe to read from the bridge observability path.
func (s *Shell) DedupDropsSnapshot() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dedupDrops
}

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

func (s *Shell) doHTTPAPIRequest(ctx context.Context, request apiCallRequest) (apiResponse, error) {
	snapshot := s.Snapshot()
	endpoint := strings.TrimSpace(s.cfg.HTTPAPI.URL)
	if endpoint == "" || !snapshot.HTTPAPI.Enabled || !snapshot.HTTPAPI.Configured {
		return apiResponse{}, errorf(errorCodeConnectionLost, "adapter transport is not connected", nil)
	}

	body, err := json.Marshal(request)
	if err != nil {
		return apiResponse{}, errorf(errorCodeHTTPAPIInvalidResponse, "encode OneBot HTTP request failed", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return apiResponse{}, errorf(errorCodeHTTPAPIRequestFailed, "build OneBot HTTP request failed", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if accessToken := strings.TrimSpace(s.cfg.HTTPAPI.AccessToken); accessToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		s.markTransportFailure(TransportHTTPAPI, TransportStateReconnecting, errorCodeHTTPAPIRequestFailed, err)
		return apiResponse{}, errorf(errorCodeHTTPAPIRequestFailed, "OneBot HTTP API request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		s.markTransportFailure(TransportHTTPAPI, TransportStateAuthFailed, errorCodeHTTPAPIAuthFailed, fmt.Errorf("status %d", resp.StatusCode))
		return apiResponse{}, errorf(errorCodeHTTPAPIAuthFailed, "OneBot HTTP API authentication failed", nil)
	}

	var decoded struct {
		Status  any    `json:"status"`
		RetCode int    `json:"retcode"`
		Wording string `json:"wording"`
		Data    any    `json:"data"`
		Echo    any    `json:"echo"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		s.markTransportFailure(TransportHTTPAPI, TransportStateReconnecting, errorCodeHTTPAPIInvalidResponse, err)
		return apiResponse{}, errorf(errorCodeHTTPAPIInvalidResponse, "OneBot HTTP API response is invalid", err)
	}

	s.mu.Lock()
	s.snapshot.HTTPAPI.State = TransportStateConnected
	s.snapshot.HTTPAPI.LastErrorCode = ""
	s.snapshot.HTTPAPI.LastErrorMessage = ""
	s.syncLastErrorLocked()
	s.refreshAggregateStateLocked()
	snapshot = cloneSnapshot(s.snapshot)
	handler := s.stateHandler
	s.mu.Unlock()
	s.emitStateSnapshot(handler, snapshot)

	echo, _ := frameEcho(decoded.Echo)
	return apiResponse{
		Echo:    echo,
		Status:  frameStatusText(decoded.Status),
		RetCode: decoded.RetCode,
		Wording: strings.TrimSpace(decoded.Wording),
		Data:    decoded.Data,
	}, nil
}
