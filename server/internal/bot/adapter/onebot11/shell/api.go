package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	adapterapi "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/api"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
)

var apiBestEffortTransportTimeout = 2 * time.Second

// callAPI sends a generic OneBot11 API request and waits for the matched
// response. It reuses the same echo-based request/response infrastructure
// that outbound uses for send_msg.
func (s *Shell) callAPI(ctx context.Context, action string, params map[string]any) (map[string]any, error) {
	responseData, err := s.CallAPIAny(ctx, action, params)
	if err != nil {
		return nil, err
	}
	data, ok := responseData.(map[string]any)
	if !ok {
		return nil, errorf(adapterapi.ErrorCodeAPICallFailed, fmt.Sprintf("%s returned a non-object payload", action), nil)
	}
	return data, nil
}

func (s *Shell) callAPIOnTransport(ctx context.Context, transport TransportKey, action string, params map[string]any) (map[string]any, error) {
	responseData, err := s.callAPIAnyOnTransport(ctx, transport, action, params)
	if err != nil {
		return nil, err
	}
	data, ok := responseData.(map[string]any)
	if !ok {
		return nil, errorf(adapterapi.ErrorCodeAPICallFailed, fmt.Sprintf("%s returned a non-object payload", action), nil)
	}
	return data, nil
}

func (s *Shell) CallAPIAny(ctx context.Context, action string, params map[string]any) (any, error) {
	return s.callAPIAnyOnTransport(ctx, "", action, params)
}

func (s *Shell) callAPIAnyBestEffort(ctx context.Context, action string, params map[string]any) (any, error) {
	transports := s.apiCandidateTransports()
	if len(transports) == 0 {
		return s.CallAPIAny(ctx, action, params)
	}

	var lastErr error
	for _, transport := range transports {
		attemptCtx, cancel, err := apiBestEffortAttemptContext(ctx)
		if err != nil {
			return nil, err
		}
		result, err := s.callAPIAnyOnTransport(attemptCtx, transport, action, params)
		cancel()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, lastErr
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errorf(errorCodeConnectionLost, "adapter transport is not connected", nil)
}

func (s *Shell) apiCandidateTransports() []TransportKey {
	snapshot := s.Snapshot()
	transports := make([]TransportKey, 0, 3)
	if snapshot.ForwardWS.State == TransportStateConnected {
		transports = append(transports, TransportForwardWS)
	}
	if snapshot.ReverseWS.State == TransportStateConnected {
		transports = append(transports, TransportReverseWS)
	}
	if snapshot.HTTPAPI.Enabled && snapshot.HTTPAPI.Configured {
		transports = append(transports, TransportHTTPAPI)
	}
	return transports
}

func apiBestEffortAttemptContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	timeout := apiBestEffortTransportTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, errorf(adapterapi.ErrorCodeAPICallFailed, "api request timed out", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, nil, errorf(adapterapi.ErrorCodeAPICallFailed, "api request timed out", ctx.Err())
		}
		if remaining < timeout {
			attemptCtx, cancel := context.WithTimeout(ctx, remaining)
			return attemptCtx, cancel, nil
		}
	}
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	return attemptCtx, cancel, nil
}

func (s *Shell) callAPIAnyOnTransport(ctx context.Context, transport TransportKey, action string, params map[string]any) (any, error) {
	echo := s.nextRequestEcho()
	request := adapteroutbound.APICallRequest{
		Action: action,
		Params: params,
		Echo:   echo,
	}

	if transport != "" {
		switch transport {
		case TransportForwardWS, TransportReverseWS:
			conn, _, snapshot := s.currentWSConnForTransport(transport)
			if conn == nil || snapshot.State != StateConnected {
				return nil, errorf(errorCodeConnectionLost, "adapter transport is not connected", nil)
			}
			responseCh := make(chan adapteroutbound.APIResponse, 1)
			s.registerPendingResponse(echo, responseCh)
			defer s.dropPendingResponse(echo)

			s.sendMu.Lock()
			writeErr := wsjsonWrite(ctx, conn, request)
			s.sendMu.Unlock()
			if writeErr != nil {
				return nil, errorf(adapterapi.ErrorCodeAPICallFailed, fmt.Sprintf("%s request failed", action), writeErr)
			}

			select {
			case response := <-responseCh:
				if response.Status != "ok" || response.RetCode != 0 {
					message := fmt.Sprintf("%s call failed", action)
					if response.Wording != "" {
						message = response.Wording
					}
					return nil, errorf(adapterapi.ErrorCodeAPICallFailed, message, nil)
				}
				result := normalizeAPIResult(response.Data)
				s.invalidateIdentityCacheForAPICall(action, params)
				return result, nil
			case <-ctx.Done():
				return nil, errorf(adapterapi.ErrorCodeAPICallFailed, fmt.Sprintf("%s response timed out", action), ctx.Err())
			}
		case TransportHTTPAPI:
			response, err := s.doHTTPAPIRequest(ctx, request)
			if err != nil {
				return nil, err
			}
			if response.Status != "ok" || response.RetCode != 0 {
				message := fmt.Sprintf("%s call failed", action)
				if response.Wording != "" {
					message = response.Wording
				}
				return nil, errorf(adapterapi.ErrorCodeAPICallFailed, message, nil)
			}
			result := normalizeAPIResult(response.Data)
			s.invalidateIdentityCacheForAPICall(action, params)
			return result, nil
		default:
			return nil, errorf(errorCodeConnectionLost, "adapter transport is not connected", nil)
		}
	}

	conn, _, snapshot := s.currentWSConn()
	if conn != nil && snapshot.State == StateConnected {
		responseCh := make(chan adapteroutbound.APIResponse, 1)
		s.registerPendingResponse(echo, responseCh)
		defer s.dropPendingResponse(echo)

		s.sendMu.Lock()
		writeErr := wsjsonWrite(ctx, conn, request)
		s.sendMu.Unlock()
		if writeErr != nil {
			return nil, errorf(adapterapi.ErrorCodeAPICallFailed, fmt.Sprintf("write %s request", action), writeErr)
		}

		select {
		case response := <-responseCh:
			if response.Status != "ok" || response.RetCode != 0 {
				message := fmt.Sprintf("%s call failed", action)
				if response.Wording != "" {
					message = response.Wording
				}
				return nil, errorf(adapterapi.ErrorCodeAPICallFailed, message, nil)
			}
			result := normalizeAPIResult(response.Data)
			s.invalidateIdentityCacheForAPICall(action, params)
			return result, nil
		case <-ctx.Done():
			return nil, errorf(adapterapi.ErrorCodeAPICallFailed, fmt.Sprintf("%s response timed out", action), ctx.Err())
		}
	}

	response, err := s.doHTTPAPIRequest(ctx, request)
	if err != nil {
		return nil, err
	}
	if response.Status != "ok" || response.RetCode != 0 {
		message := fmt.Sprintf("%s call failed", action)
		if response.Wording != "" {
			message = response.Wording
		}
		return nil, errorf(adapterapi.ErrorCodeAPICallFailed, message, nil)
	}
	result := normalizeAPIResult(response.Data)
	s.invalidateIdentityCacheForAPICall(action, params)
	return result, nil
}

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
