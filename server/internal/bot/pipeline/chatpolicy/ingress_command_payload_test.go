package chatpolicy_test

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/bot/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestEnrichCommandEventAddsCommandPayload(t *testing.T) {
	t.Parallel()

	testConfig := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/", "!"},
		},
	}
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	event := ingress.EnrichCommandEvent(chatevent.NormalizedEvent{
		PlainText: "/weather shanghai now",
	})
	if event.PayloadFields["command"] != "weather" {
		t.Fatalf("unexpected command payload: %#v", event.PayloadFields)
	}
	args, ok := event.PayloadFields["args"].([]string)
	if !ok {
		t.Fatalf("unexpected args payload type: %#v", event.PayloadFields["args"])
	}
	if len(args) != 2 || args[0] != "shanghai" || args[1] != "now" {
		t.Fatalf("unexpected args payload: %#v", args)
	}
}
