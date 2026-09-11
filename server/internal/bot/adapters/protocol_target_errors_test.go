package adapters

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

func TestTargetFailureClassificationIgnoresTranslatedErrorMessages(t *testing.T) {
	const fallback = "fixture fallback"
	for _, code := range []string{errorcodes.AdapterConnectionLost, errorcodes.AdapterConnectionFailed} {
		first := oneBot11TargetIssueMessage(fallback, &onebot11.Error{Code: code, Message: "first language"})
		second := oneBot11TargetIssueMessage(fallback, &onebot11.Error{Code: code, Message: "另一种语言"})
		if first != second || first == fallback {
			t.Fatalf("code %s was classified by message: %q / %q", code, first, second)
		}
	}
	for _, cause := range []error{context.DeadlineExceeded, onebot11.ErrInvalidListPayload} {
		first := oneBot11TargetIssueMessage(fallback, fmt.Errorf("first language: %w", cause))
		second := oneBot11TargetIssueMessage(fallback, fmt.Errorf("另一种语言: %w", cause))
		if first != second || first == fallback {
			t.Fatalf("wrapped cause was classified by message: %q / %q", first, second)
		}
	}
	for _, text := range []string{"not connected", "timed out", "non-list payload"} {
		if got := oneBot11TargetIssueMessage(fallback, errors.New(text)); got != fallback {
			t.Fatalf("untyped text selected an operational diagnosis: %q", got)
		}
	}
}
