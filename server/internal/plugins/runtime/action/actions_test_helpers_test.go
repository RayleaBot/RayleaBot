package action

import (
	"errors"
	"testing"
)

func assertActionErrorCode(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected action error %q, got nil", want)
	}
	var actionErr *Error
	if !errors.As(err, &actionErr) {
		t.Fatalf("expected *action.Error, got %T", err)
	}
	if actionErr.Code != want {
		t.Fatalf("unexpected error code: got %q want %q", actionErr.Code, want)
	}
}
