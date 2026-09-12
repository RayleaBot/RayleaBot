package runtime

import (
	"errors"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func assertProtocolViolation(t *testing.T, err error) {
	const want = codePluginProtocolViolation

	t.Helper()

	if err == nil {
		t.Fatalf("expected action error %q, got nil", want)
	}
	var actionErr *plugins.Error
	if !errors.As(err, &actionErr) {
		t.Fatalf("expected *action.Error, got %T", err)
	}
	if actionErr.Code != want {
		t.Fatalf("unexpected error code: got %q want %q", actionErr.Code, want)
	}
}
