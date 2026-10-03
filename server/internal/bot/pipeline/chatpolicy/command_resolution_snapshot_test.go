package chatpolicy_test

import (
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestApplyKeepsCommandTargetsAndPermissionFromOneCatalogSnapshot(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		name := "ordinary"
		if fallback {
			name = "fallback"
		}
		t.Run(name, func(t *testing.T) {
			catalog := &changingPolicyCatalog{entries: []plugins.CommandEntry{{
				PluginID: "restricted", Commands: []plugins.Command{{Name: "echo", Permission: "group_admin", Fallback: fallback}},
			}}}
			cfg := config.Config{Command: &config.CommandConfig{Prefixes: []string{"/"}}}
			service := chatpolicy.New(chatpolicy.Deps{CurrentConfig: func() config.Config { return cfg }, Plugins: catalog})
			catalog.onRead = func() {
				catalog.entries = []plugins.CommandEntry{{PluginID: "public", Commands: []plugins.Command{{Name: "echo", Permission: "everyone"}}}}
			}

			first, allowed := service.Apply(t.Context(), policySnapshotMessage("/echo hello"))
			if allowed || len(first.CommandTargets) != 1 || first.CommandTargets[0].PluginID != "restricted" {
				t.Fatalf("old catalog admission: allowed=%v targets=%+v", allowed, first.CommandTargets)
			}
			second, allowed := service.Apply(t.Context(), policySnapshotMessage("/echo hello"))
			if !allowed || len(second.CommandTargets) != 1 || second.CommandTargets[0].PluginID != "public" {
				t.Fatalf("new catalog admission: allowed=%v targets=%+v", allowed, second.CommandTargets)
			}
		})
	}
}

func TestApplyKeepsPrefixesAndPermissionFromOneConfigSnapshot(t *testing.T) {
	cfg := config.Config{
		Command:    &config.CommandConfig{Prefixes: []string{"/"}},
		Permission: config.PermissionConfig{DefaultLevel: "group_admin"},
	}
	catalog := &changingPolicyCatalog{entries: []plugins.CommandEntry{{PluginID: "echo", Commands: []plugins.Command{{Name: "echo"}}}}}
	service := chatpolicy.New(chatpolicy.Deps{CurrentConfig: func() config.Config { return cfg }, Plugins: catalog})
	catalog.onRead = func() {
		cfg = config.Config{
			Command:    &config.CommandConfig{Prefixes: []string{"!"}},
			Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		}
		service.UpdateConfig(cfg)
	}

	first, allowed := service.Apply(t.Context(), policySnapshotMessage("/echo hello"))
	if allowed || len(first.CommandTargets) != 1 || first.CommandTargets[0].PluginID != "echo" {
		t.Fatalf("old config admission: allowed=%v targets=%+v", allowed, first.CommandTargets)
	}
	second, allowed := service.Apply(t.Context(), policySnapshotMessage("!echo hello"))
	if !allowed || len(second.CommandTargets) != 1 || second.CommandTargets[0].PluginID != "echo" {
		t.Fatalf("new config admission: allowed=%v targets=%+v", allowed, second.CommandTargets)
	}
	if event := service.EnrichCommandEvent(policySnapshotMessage("/echo hello")); event.CommandResolved {
		t.Fatalf("old prefix still resolves after reload: %+v", event.CommandTargets)
	}
	if info := service.CommandInfoForEvent(second); info == nil || info.Permission != "everyone" {
		t.Fatalf("independent command info did not use the new config: %+v", info)
	}
}

func TestApplyKeepsInputAndEachCommandTargetArgsIndependent(t *testing.T) {
	cfg := config.Config{Command: &config.CommandConfig{Prefixes: []string{"/"}}}
	catalog := &changingPolicyCatalog{entries: []plugins.CommandEntry{
		{PluginID: "first", Commands: []plugins.Command{{Name: "echo"}}},
		{PluginID: "second", Commands: []plugins.Command{{Name: "echo"}}},
	}}
	service := chatpolicy.New(chatpolicy.Deps{CurrentConfig: func() config.Config { return cfg }, Plugins: catalog})
	input := policySnapshotMessage("/echo hello")
	input.PayloadFields = map[string]any{"command": "upstream", "args": []string{"upstream-argument"}, "retained": true}
	enriched, allowed := service.Apply(t.Context(), input)
	if !allowed || len(enriched.CommandTargets) != 2 {
		t.Fatalf("admission: allowed=%v targets=%+v", allowed, enriched.CommandTargets)
	}
	if !reflect.DeepEqual(input.PayloadFields, map[string]any{"command": "upstream", "args": []string{"upstream-argument"}, "retained": true}) || input.CommandResolved || len(input.CommandTargets) != 0 {
		t.Fatalf("input was mutated: %+v", input)
	}
	if enriched.PayloadFields["retained"] != true || enriched.PayloadFields["command"] != "echo" {
		t.Fatalf("unexpected command payload: %+v", enriched.PayloadFields)
	}
	enriched.PayloadFields["args"].([]string)[0] = "payload mutation"
	if enriched.CommandTargets[0].Args[0] != "hello" || enriched.CommandTargets[1].Args[0] != "hello" {
		t.Fatalf("payload args alias command target args: %+v", enriched.CommandTargets)
	}
	enriched.CommandTargets[0].Args[0] = "target mutation"
	if enriched.CommandTargets[1].Args[0] != "hello" {
		t.Fatalf("command targets share args: %+v", enriched.CommandTargets)
	}
}

func policySnapshotMessage(text string) chatevent.NormalizedEvent {
	return chatevent.NormalizedEvent{
		Kind: chatevent.EventKindMessage, SourceProtocol: "onebot11", SourceAdapter: "fixture",
		EventType: "message.group", SenderID: "member", ActorRole: "member",
		ConversationType: "group", ConversationID: "group", PlainText: text,
	}
}

// The hook models a reload immediately after a reader acquires the immutable
// catalog slice, without relying on scheduler timing or mutating that slice.
type changingPolicyCatalog struct {
	entries []plugins.CommandEntry
	onRead  func()
}

func (*changingPolicyCatalog) List() []plugins.Snapshot { return nil }

func (c *changingPolicyCatalog) Commands() []plugins.CommandEntry {
	entries := c.entries
	if hook := c.onRead; hook != nil {
		c.onRead = nil
		hook()
	}
	return entries
}
