package outbound

import (
	"context"
	"errors"
	"testing"
)

type fakeTransport struct {
	echo              string
	wsResponse        APIResponse
	wsOK              bool
	wsErr             error
	httpResponse      APIResponse
	httpErr           error
	wsRequests        []SendMsgRequest
	httpRequests      []APICallRequest
	unsupportedLogged []string
}

func (t *fakeTransport) NextEcho() string {
	if t.echo == "" {
		return "adapter-test"
	}
	return t.echo
}

func (t *fakeTransport) SendWebSocket(_ context.Context, request SendMsgRequest) (APIResponse, bool, error) {
	t.wsRequests = append(t.wsRequests, request)
	return t.wsResponse, t.wsOK, t.wsErr
}

func (t *fakeTransport) DoHTTPAPI(_ context.Context, request APICallRequest) (APIResponse, error) {
	t.httpRequests = append(t.httpRequests, request)
	return t.httpResponse, t.httpErr
}

func (t *fakeTransport) LogUnsupportedSegment(segmentType string) {
	t.unsupportedLogged = append(t.unsupportedLogged, segmentType)
}

func TestSenderSendMessageUsesWebSocketAndLogsUnsupportedSegments(t *testing.T) {
	transport := &fakeTransport{
		echo: "adapter-1",
		wsOK: true,
		wsResponse: APIResponse{
			Status:  "ok",
			RetCode: 0,
			Data:    map[string]any{"message_id": float64(123)},
		},
	}

	result, err := NewSender(transport).SendMessage(context.Background(), OutboundMessageSend{
		TargetType: "group",
		TargetID:   "10001",
		Segments: []OutboundMessageSegment{
			{Type: "text", Data: map[string]any{"text": "hello"}},
			{Type: "unsupported", Data: map[string]any{"value": "drop"}},
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if result.MessageID != "123" {
		t.Fatalf("unexpected message id: got %q want %q", result.MessageID, "123")
	}
	if len(transport.wsRequests) != 1 {
		t.Fatalf("expected one websocket request, got %d", len(transport.wsRequests))
	}
	request := transport.wsRequests[0]
	if request.Action != "send_msg" || request.Echo != "adapter-1" {
		t.Fatalf("unexpected request envelope: %#v", request)
	}
	if request.Params.MessageType != "group" || request.Params.GroupID != int64(10001) {
		t.Fatalf("unexpected target params: %#v", request.Params)
	}
	segments, ok := request.Params.Message.([]OneBotMessageSegment)
	if !ok || len(segments) != 1 || segments[0].Type != "text" || segments[0].Data["text"] != "hello" {
		t.Fatalf("unexpected message segments: %#v", request.Params.Message)
	}
	if len(transport.unsupportedLogged) != 1 || transport.unsupportedLogged[0] != "unsupported" {
		t.Fatalf("unexpected unsupported segment log: %#v", transport.unsupportedLogged)
	}
}

func TestSenderSendMessageFallsBackToHTTPAPI(t *testing.T) {
	transport := &fakeTransport{
		echo: "adapter-2",
		httpResponse: APIResponse{
			Status:  "ok",
			RetCode: 0,
			Data:    map[string]any{"message_id": "abc"},
		},
	}

	result, err := NewSender(transport).SendMessage(context.Background(), OutboundMessageSend{
		TargetType: "private",
		TargetID:   "u-1",
		Segments: []OutboundMessageSegment{
			{Type: "text", Data: map[string]any{"text": "hello"}},
		},
	})
	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if result.MessageID != "abc" {
		t.Fatalf("unexpected message id: got %q want %q", result.MessageID, "abc")
	}
	if len(transport.httpRequests) != 1 {
		t.Fatalf("expected one HTTP API request, got %d", len(transport.httpRequests))
	}
	request := transport.httpRequests[0]
	if request.Action != "send_msg" || request.Params["message_type"] != "private" || request.Params["user_id"] != "u-1" {
		t.Fatalf("unexpected HTTP API request: %#v", request)
	}
}

func TestSenderSendReplyMapsMissingTarget(t *testing.T) {
	transport := &fakeTransport{
		wsOK: true,
		wsResponse: APIResponse{
			Status:  "failed",
			RetCode: 1200,
			Wording: "消息不存在",
		},
	}

	_, err := NewSender(transport).SendReply(context.Background(), OutboundMessageReply{
		TargetType:       "private",
		TargetID:         "42",
		ReplyToMessageID: "7",
		Segments: []OutboundMessageSegment{
			{Type: "text", Data: map[string]any{"text": "reply"}},
		},
	})
	if err == nil {
		t.Fatal("expected SendReply to fail")
	}
	var adapterErr *Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected adapter error, got %T", err)
	}
	if adapterErr.Code != ErrorCodeReplyTargetMissing {
		t.Fatalf("unexpected error code: got %q want %q", adapterErr.Code, ErrorCodeReplyTargetMissing)
	}
}
