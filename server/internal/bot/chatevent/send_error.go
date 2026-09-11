package chatevent

import (
	"context"
	"errors"
	"fmt"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

// SendError is the protocol-neutral failure returned at a message sender's
// boundary. Err preserves internal transport evidence; Message is safe for the
// plugin action response. An unconfirmed send must never be retried implicitly.
type SendError struct {
	Code    string
	Message string
	Err     error
}

func (e *SendError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return e.Code + ": " + e.Message
}
func (e *SendError) Unwrap() error                { return e.Err }
func (e *SendError) RuntimeActionCode() string    { return e.Code }
func (e *SendError) RuntimeActionMessage() string { return e.Message }

func SendErrorCode(err error) string {
	var failure *SendError
	if errors.As(err, &failure) && failure != nil {
		return failure.Code
	}
	return ""
}

// SendOutcome returns the finite vocabulary used by outbound observations.
func SendOutcome(err error) string {
	if err == nil {
		return "delivered"
	}
	// Delivery uncertainty takes precedence over its underlying timeout/cancel.
	if SendErrorCode(err) == errorcodes.AdapterSendUnconfirmed {
		return "unconfirmed"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	switch SendErrorCode(err) {
	case errorcodes.PluginPermissionDenied, errorcodes.PermissionDenied, errorcodes.AdapterAuthFailed:
		return "permission_denied"
	case errorcodes.AdapterReplyTargetMissing:
		return "reply_target_missing"
	case errorcodes.PlatformRateLimited, errorcodes.AdapterMessageQuotaExceeded:
		return "rate_limited"
	case errorcodes.AdapterConnectionFailed, errorcodes.AdapterConnectionLost, errorcodes.AdapterTransportUnavailable:
		return "not_connected"
	default:
		return "failed"
	}
}

func ProtocolLabel(protocol string) string {
	switch protocol {
	case "onebot11", "qqofficial":
		return protocol
	default:
		return "unknown"
	}
}
