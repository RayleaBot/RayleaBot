package dispatch

import (
	"context"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
)

func TestSchedulerTimeoutOutcomeUsesRegisteredCodes(t *testing.T) {
	for _, code := range []string{errorcodes.PlatformRenderTimeout, errorcodes.PlatformTaskTimeout, errorcodes.PluginInitTimeout, errorcodes.PluginEventTimeout, errorcodes.PluginShutdownTimeout} {
		outcome, actual, _ := schedulerFailureFields(nil, plugins.Delivery{ErrorCode: code, ErrorMessage: "任意语言"})
		if outcome != scheduler.RunOutcomeTimeout || actual != code {
			t.Fatalf("registered timeout %s: %s / %s", code, outcome, actual)
		}
	}
	for _, code := range []string{"thirdparty.timeout_note", "unexpected.timeout", errorcodes.PluginInternalError} {
		outcome, _, _ := schedulerFailureFields(nil, plugins.Delivery{ErrorCode: code, ErrorMessage: "timeout"})
		if outcome != scheduler.RunOutcomeFailed {
			t.Fatalf("arbitrary code/message selected timeout outcome: %s", code)
		}
	}
	if outcome, code, _ := schedulerFailureFields(context.Canceled, plugins.Delivery{}); outcome != scheduler.RunOutcomeOther || code != errorcodes.PluginEventCanceled {
		t.Fatalf("cancellation classification: %s / %s", outcome, code)
	}
}
