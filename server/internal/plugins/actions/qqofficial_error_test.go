package actions

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/qqofficial"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestQQErrorSurvivesPluginRPCBoundary(t *testing.T) {
	for _, code := range []string{qqofficial.CodeMessageQuotaExceeded, qqofficial.CodeReplyWindowExpired, qqofficial.CodeCapabilityUnsupported, qqofficial.CodeSendUnconfirmed} {
		t.Run(code, func(t *testing.T) {
			got := oneBotRuntimeActionError(&chatevent.SendError{Code: code, Message: "fixture"}).(*plugins.Error)
			if got.Code != code {
				t.Fatalf("plugin sees %s, want %s", got.Code, code)
			}
		})
	}
}
