package system

import (
	"errors"
	"fmt"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

func TestRuntimeInspectionUsesResourceCauseInsteadOfMessage(t *testing.T) {
	for _, message := range []string{"first language", "不同语言"} {
		err := &deps.BootstrapError{Message: message, Err: fmt.Errorf("%s: %w", message, deps.ErrResourceNotDeclared)}
		if got := startupInspectionIssue("ffmpeg", err); got.Code != errorcodes.DiagnosticDepsManifestPlatformMissing {
			t.Fatalf("resource declaration failure classified as %s", got.Code)
		}
	}
	for _, err := range []error{errors.New("does not include"), &deps.BootstrapError{Err: errors.New("does not include")}, &deps.BootstrapError{Message: "no inner cause"}} {
		if got := startupInspectionIssue("ffmpeg", err); got.Code != errorcodes.DiagnosticDepsManifestMissing {
			t.Fatalf("untyped wording selected resource declaration diagnosis: %s", got.Code)
		}
	}
}
