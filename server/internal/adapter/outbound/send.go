package outbound

import (
	"context"
	"strconv"
)

func (s Sender) sendSegments(ctx context.Context, targetType, targetID string, segments []OneBotMessageSegment, replyAttempt bool) (SendMessageResult, error) {
	if s.transport == nil {
		return SendMessageResult{}, Errorf(ErrorCodeSendFailed, "adapter transport is not connected", nil)
	}

	echo := s.transport.NextEcho()
	request := SendMsgRequest{
		Action: "send_msg",
		Params: SendMsgParams{
			MessageType: targetType,
			Message:     segments,
		},
		Echo: echo,
	}
	switch targetType {
	case "group":
		request.Params.GroupID = OneBotTargetValue(targetID)
	case "private":
		request.Params.UserID = OneBotTargetValue(targetID)
	}

	if response, ok, err := s.transport.SendWebSocket(ctx, request); ok || err != nil {
		if err != nil {
			return SendMessageResult{}, err
		}
		return ParseSendMessageResponse(response, replyAttempt)
	}

	params := map[string]any{
		"message_type": targetType,
		"message":      segments,
	}
	if request.Params.UserID != nil {
		params["user_id"] = request.Params.UserID
	}
	if request.Params.GroupID != nil {
		params["group_id"] = request.Params.GroupID
	}
	response, err := s.transport.DoHTTPAPI(ctx, APICallRequest{
		Action: request.Action,
		Params: params,
		Echo:   request.Echo,
	})
	if err != nil {
		return SendMessageResult{}, err
	}
	return ParseSendMessageResponse(response, replyAttempt)
}

func OneBotTargetValue(targetID string) any {
	if value, err := strconv.ParseInt(targetID, 10, 64); err == nil {
		return value
	}
	return targetID
}
