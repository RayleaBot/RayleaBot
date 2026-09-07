package actions

import (
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/qqofficial"
	"testing"
)

func TestQQErrorSurvivesPluginRPCBoundary(t *testing.T) {
	for _, code := range []string{qqofficial.CodeMessageQuotaExceeded, qqofficial.CodeReplyWindowExpired, qqofficial.CodeCapabilityUnsupported} {
		t.Run(code, func(t *testing.T) {
			got := oneBotRuntimeActionError(&qqofficial.SendError{Code: code, Message: "fixture"}).(*pluginruntime.Error)
			if got.Code != code {
				t.Fatalf("plugin sees %s, want %s", got.Code, code)
			}
		})
	}
}
