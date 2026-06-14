package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/adapter/outbound"
)

func (s *Shell) doHTTPAPIRequest(ctx context.Context, request adapteroutbound.APICallRequest) (adapteroutbound.APIResponse, error) {
	snapshot := s.Snapshot()
	endpoint := strings.TrimSpace(s.cfg.HTTPAPI.URL)
	if endpoint == "" || !snapshot.HTTPAPI.Enabled || !snapshot.HTTPAPI.Configured {
		return adapteroutbound.APIResponse{}, errorf(errorCodeConnectionLost, "adapter transport is not connected", nil)
	}

	body, err := json.Marshal(request)
	if err != nil {
		return adapteroutbound.APIResponse{}, errorf(errorCodeHTTPAPIInvalidResponse, "encode OneBot HTTP request failed", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return adapteroutbound.APIResponse{}, errorf(errorCodeHTTPAPIRequestFailed, "build OneBot HTTP request failed", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if accessToken := strings.TrimSpace(s.cfg.HTTPAPI.AccessToken); accessToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		s.markTransportFailure(TransportHTTPAPI, TransportStateReconnecting, errorCodeHTTPAPIRequestFailed, err)
		return adapteroutbound.APIResponse{}, errorf(errorCodeHTTPAPIRequestFailed, "OneBot HTTP API request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		s.markTransportFailure(TransportHTTPAPI, TransportStateAuthFailed, errorCodeHTTPAPIAuthFailed, fmt.Errorf("status %d", resp.StatusCode))
		return adapteroutbound.APIResponse{}, errorf(errorCodeHTTPAPIAuthFailed, "OneBot HTTP API authentication failed", nil)
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
		return adapteroutbound.APIResponse{}, errorf(errorCodeHTTPAPIInvalidResponse, "OneBot HTTP API response is invalid", err)
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
	return adapteroutbound.APIResponse{
		Echo:    echo,
		Status:  frameStatusText(decoded.Status),
		RetCode: decoded.RetCode,
		Wording: strings.TrimSpace(decoded.Wording),
		Data:    decoded.Data,
	}, nil
}
