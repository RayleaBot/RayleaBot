package services

import (
	"errors"
	"testing"

	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
)

func assertRuntimeErrorCode(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected runtime error %q, got nil", want)
	}

	var runtimeErr *pluginruntime.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected *pluginruntime.Error, got %T", err)
	}
	if runtimeErr.Code != want {
		t.Fatalf("unexpected runtime error code: got %q want %q", runtimeErr.Code, want)
	}
}
