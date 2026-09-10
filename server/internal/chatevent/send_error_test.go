package chatevent

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
)

func TestSendOutcomeKeepsDeliveryUncertaintyAheadOfTransportCause(t *testing.T) {
	for _, tc := range []struct {
		code    string
		cause   error
		outcome string
	}{
		{errorcodes.AdapterSendUnconfirmed, context.DeadlineExceeded, "unconfirmed"},
		{errorcodes.AdapterSendFailed, context.DeadlineExceeded, "timeout"},
		{errorcodes.AdapterSendFailed, context.Canceled, "canceled"},
		{errorcodes.AdapterAuthFailed, nil, "permission_denied"},
		{errorcodes.AdapterMessageQuotaExceeded, nil, "rate_limited"},
		{errorcodes.AdapterConnectionLost, nil, "not_connected"},
		{errorcodes.AdapterReplyTargetMissing, nil, "reply_target_missing"},
	} {
		err := fmt.Errorf("outer: %w", &SendError{Code: tc.code, Message: "variable platform wording", Err: tc.cause})
		if got := SendOutcome(err); got != tc.outcome {
			t.Errorf("%s: %s", tc.code, got)
		}
		if tc.cause != nil && !errors.Is(err, tc.cause) {
			t.Fatal("transport cause was lost")
		}
	}
	if ProtocolLabel("arbitrary-instance-id") != "unknown" {
		t.Fatal("unbounded metric label")
	}
}
