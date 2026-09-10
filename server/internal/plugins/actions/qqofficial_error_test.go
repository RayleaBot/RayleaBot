package actions

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/qqofficial"
)

func TestQQErrorSurvivesPluginRPCBoundary(t *testing.T) {
	for _, code := range []string{qqofficial.CodeMessageQuotaExceeded, qqofficial.CodeReplyWindowExpired, qqofficial.CodeCapabilityUnsupported, qqofficial.CodeSendUnconfirmed} {
		t.Run(code, func(t *testing.T) {
			got := oneBotRuntimeActionError(&qqofficial.SendError{Code: code, Message: "fixture"}).(*plugins.Error)
			if got.Code != code {
				t.Fatalf("plugin sees %s, want %s", got.Code, code)
			}
		})
	}
}
