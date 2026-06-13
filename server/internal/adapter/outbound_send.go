package adapter

import (
	"context"
	"strconv"
)

func (s *Shell) sendSegments(ctx context.Context, targetType, targetID string, segments []oneBotMessageSegment, replyAttempt bool) (SendMessageResult, error) {
	echo := s.nextRequestEcho()
	request := sendMsgRequest{
		Action: "send_msg",
		Params: sendMsgParams{
			MessageType: targetType,
			Message:     segments,
		},
		Echo: echo,
	}
	switch targetType {
	case "group":
		request.Params.GroupID = oneBotTargetValue(targetID)
	case "private":
		request.Params.UserID = oneBotTargetValue(targetID)
	}

	conn, _, snapshot := s.currentWSConn()
	if conn != nil && snapshot.State == StateConnected {
		responseCh := make(chan apiResponse, 1)
		s.registerPendingResponse(echo, responseCh)
		defer s.dropPendingResponse(echo)

		s.sendMu.Lock()
		writeErr := wsjsonWrite(ctx, conn, request)
		s.sendMu.Unlock()
		if writeErr != nil {
			return SendMessageResult{}, errorf(errorCodeSendFailed, "write send_msg request", writeErr)
		}

		select {
		case response := <-responseCh:
			return parseSendMessageResponse(response, replyAttempt)
		case <-ctx.Done():
			return SendMessageResult{}, errorf(errorCodeSendFailed, "adapter send_msg response timed out", ctx.Err())
		}
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
	response, err := s.doHTTPAPIRequest(ctx, apiCallRequest{
		Action: request.Action,
		Params: params,
		Echo:   request.Echo,
	})
	if err != nil {
		return SendMessageResult{}, err
	}
	return parseSendMessageResponse(response, replyAttempt)
}

func oneBotTargetValue(targetID string) any {
	if value, err := strconv.ParseInt(targetID, 10, 64); err == nil {
		return value
	}
	return targetID
}
