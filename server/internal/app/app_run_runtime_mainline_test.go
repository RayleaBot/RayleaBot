package app

import (
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestPluginDiscoveryContextIncludesBuiltinRoot(t *testing.T) {
	t.Parallel()

	_, _, roots, err := pluginDiscoveryContext(filepath.Join("..", "..", "..", "contracts", "config.user.schema.json"))
	if err != nil {
		t.Fatalf("pluginDiscoveryContext failed: %v", err)
	}
	if len(roots) == 0 || roots[0].Label != "plugins/builtin" {
		t.Fatalf("expected builtin root first, got %#v", roots)
	}
}

func TestEnrichCommandEventAddsCommandPayload(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/", "!"},
		},
	}, nil)
	application.setTestEventIngress(nil, nil, nil, nil)

	event := application.enrichCommandEvent(adapter.NormalizedEvent{
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
