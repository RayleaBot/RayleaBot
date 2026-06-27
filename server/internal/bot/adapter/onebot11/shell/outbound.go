package shell

import (
	"context"
	"fmt"

	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
)

func errorf(code, message string, err error) *adapteroutbound.Error {
	return adapteroutbound.Errorf(code, message, err)
}

func oneBotTargetValue(targetID string) any {
	return adapteroutbound.OneBotTargetValue(targetID)
}

func (s *Shell) SendMessage(ctx context.Context, action adapteroutbound.OutboundMessageSend) (adapteroutbound.SendMessageResult, error) {
	return adapteroutbound.NewSender(shellOutboundTransport{s: s}).SendMessage(ctx, action)
}

func (s *Shell) SendReply(ctx context.Context, action adapteroutbound.OutboundMessageReply) (adapteroutbound.SendMessageResult, error) {
	return adapteroutbound.NewSender(shellOutboundTransport{s: s}).SendReply(ctx, action)
}

func (s *Shell) routeAPIResponse(frame adapterintake.ClassifiedFrame) {
	if frame.Summary.Category != adapterintake.FrameCategoryAPIResponse {
		return
	}

	response, ok := adapteroutbound.APIResponseFromFrame(adapteroutbound.FrameResponse{
		Echo:    frame.Frame.Echo,
		Status:  frame.Frame.Status,
		RetCode: frame.Frame.RetCode,
		Wording: frame.Frame.Wording,
		Data:    frame.Frame.Data,
	})
	if !ok {
		return
	}

	pending, found := s.takePendingResponse(response.Echo)
	if !found {
		s.logger.Warn(
			"OneBot API 响应没有待处理请求，已忽略：echo="+response.Echo,
			"component", "adapter",
			"adapter_state", s.Snapshot().State,
			"direction", "inbound",
			"echo", response.Echo,
			"status", response.Status,
			"retcode", response.RetCode,
			"wording", response.Wording,
			"payload_preview", response.Data,
		)
		return
	}

	select {
	case pending <- response:
	default:
	}
}

func wsjsonWrite(ctx context.Context, conn adapteroutbound.WebSocketWriter, value any) error {
	return adapteroutbound.WriteJSON(ctx, conn, value)
}

func (s *Shell) nextRequestEcho() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextEcho++
	return fmt.Sprintf("adapter-%d", s.nextEcho)
}

func (s *Shell) registerPendingResponse(echo string, responseCh chan adapteroutbound.APIResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pendingResponses[echo] = responseCh
}

func (s *Shell) dropPendingResponse(echo string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pendingResponses, echo)
}

func (s *Shell) takePendingResponse(echo string) (chan adapteroutbound.APIResponse, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	responseCh, ok := s.pendingResponses[echo]
	if ok {
		delete(s.pendingResponses, echo)
	}
	return responseCh, ok
}

type shellOutboundTransport struct {
	s *Shell
}

func (t shellOutboundTransport) NextEcho() string {
	return t.s.nextRequestEcho()
}

func (t shellOutboundTransport) SendWebSocket(ctx context.Context, request adapteroutbound.SendMsgRequest) (adapteroutbound.APIResponse, bool, error) {
	conn, _, snapshot := t.s.currentWSConn()
	if conn == nil || snapshot.State != StateConnected {
		return adapteroutbound.APIResponse{}, false, nil
	}

	responseCh := make(chan adapteroutbound.APIResponse, 1)
	t.s.registerPendingResponse(request.Echo, responseCh)
	defer t.s.dropPendingResponse(request.Echo)

	t.s.sendMu.Lock()
	writeErr := adapteroutbound.WriteJSON(ctx, conn, request)
	t.s.sendMu.Unlock()
	if writeErr != nil {
		return adapteroutbound.APIResponse{}, true, errorf(adapteroutbound.ErrorCodeSendFailed, "write send_msg request", writeErr)
	}

	select {
	case response := <-responseCh:
		return response, true, nil
	case <-ctx.Done():
		return adapteroutbound.APIResponse{}, true, errorf(adapteroutbound.ErrorCodeSendFailed, "adapter send_msg response timed out", ctx.Err())
	}
}

func (t shellOutboundTransport) DoHTTPAPI(ctx context.Context, request adapteroutbound.APICallRequest) (adapteroutbound.APIResponse, error) {
	return t.s.doHTTPAPIRequest(ctx, request)
}

func (t shellOutboundTransport) LogUnsupportedSegment(segmentType string) {
	t.s.logger.Warn(
		"OneBot 出站消息包含不支持的消息段，已丢弃：类型 "+segmentType,
		"component", "adapter",
		"segment_type", segmentType,
	)
}
