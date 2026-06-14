package shell

import (
	"context"
	"fmt"

	adapterapi "github.com/RayleaBot/RayleaBot/server/internal/adapter/api"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/adapter/outbound"
)

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
