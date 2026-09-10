package onebot11

import (
	"errors"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
)

func neutralSendError(err error) error {
	if err == nil {
		return nil
	}
	var outbound *chatevent.SendError
	if errors.As(err, &outbound) {
		return err
	}
	code, message := errorcodes.AdapterSendFailed, "消息发送失败。"
	var protocol *Error
	if errors.As(err, &protocol) {
		code, message = protocol.Code, protocol.Message
		switch code {
		case errorcodes.AdapterTransportHttpApiAuthFailed, errorcodes.AdapterTransportReverseWsAuthFailed:
			code = errorcodes.AdapterAuthFailed
		case errorcodes.AdapterTransportForwardWsConnectionFailed:
			code = errorcodes.AdapterConnectionFailed
		case errorcodes.AdapterTransportForwardWsSessionLost:
			code = errorcodes.AdapterConnectionLost
		}
	}
	return &chatevent.SendError{Code: code, Message: message, Err: err}
}
