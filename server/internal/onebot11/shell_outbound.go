package onebot11

import (
	"context"
	"fmt"
	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
)

func errorf(code, message string, err error) *Error {
	return Errorf(code, message, err)
}

func oneBotTargetValue(targetID string) any {
	return OneBotTargetValue(targetID)
}

func (s *Shell) SendMessage(ctx context.Context, action chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	result, err := NewSender(shellOutboundTransport{s: s}).SendMessage(ctx, action)
	result.SourceAdapter, result.SourceProtocol = s.adapterID, "onebot11"
	return result, err
}

func (s *Shell) SendReply(ctx context.Context, action chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	result, err := NewSender(shellOutboundTransport{s: s}).SendReply(ctx, action)
	result.SourceAdapter, result.SourceProtocol = s.adapterID, "onebot11"
	return result, err
}

func (s *Shell) routeAPIResponse(frame ClassifiedFrame) {
	if frame.Summary.Category != FrameCategoryAPIResponse {
		return
	}

	response, ok := APIResponseFromFrame(FrameResponse{
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
			"忽略无法匹配请求的消息平台回复",
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

func wsjsonWrite(ctx context.Context, conn WebSocketWriter, value any) error {
	return WriteJSON(ctx, conn, value)
}

func (s *Shell) nextRequestEcho() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextEcho++
	return fmt.Sprintf("adapter-%d", s.nextEcho)
}

func (s *Shell) registerPendingResponse(echo string, responseCh chan APIResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pendingResponses[echo] = responseCh
}

func (s *Shell) dropPendingResponse(echo string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pendingResponses, echo)
}

func (s *Shell) takePendingResponse(echo string) (chan APIResponse, bool) {
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

func (t shellOutboundTransport) SendWebSocket(ctx context.Context, request SendMsgRequest) (APIResponse, bool, error) {
	if err := ctx.Err(); err != nil {
		return APIResponse{}, true, errorf(ErrorCodeSendFailed, "发送请求已取消，消息未发出", err)
	}
	conn, _, snapshot := t.s.currentWSConn()
	if conn == nil || snapshot.State != StateConnected {
		return APIResponse{}, false, nil
	}

	responseCh := make(chan APIResponse, 1)
	t.s.registerPendingResponse(request.Echo, responseCh)
	defer t.s.dropPendingResponse(request.Echo)

	t.s.sendMu.Lock()
	if err := ctx.Err(); err != nil {
		t.s.sendMu.Unlock()
		return APIResponse{}, true, errorf(ErrorCodeSendFailed, "发送请求已取消，消息未发出", err)
	}
	writeErr := WriteJSON(ctx, conn, request)
	t.s.sendMu.Unlock()
	if writeErr != nil {
		return APIResponse{}, true, errorf(ErrorCodeSendUnconfirmed, "发送连接中断，无法确认消息是否送达；未自动重发", writeErr)
	}

	select {
	case response := <-responseCh:
		return response, true, nil
	case <-ctx.Done():
		return APIResponse{}, true, errorf(ErrorCodeSendUnconfirmed, "等待发送回执已结束，消息可能仍会送达；未自动重发", ctx.Err())
	}
}

func (t shellOutboundTransport) DoHTTPAPI(ctx context.Context, request APICallRequest) (APIResponse, error) {
	return t.s.doHTTPAPIRequest(ctx, request)
}

func (t shellOutboundTransport) LogUnsupportedSegment(segmentType string) {
	t.s.logger.Warn(
		"消息中不支持的内容已跳过，其余内容继续发送",
		"component", "adapter",
		"segment_type", segmentType,
	)
}
