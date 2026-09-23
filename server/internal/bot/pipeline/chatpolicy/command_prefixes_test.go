package chatpolicy

import (
	"context"
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/bot/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestDedicatedPrefixSelectsTargetsPermissionAndShadowsBuiltinMenu(t *testing.T) {
	enabled := func(id string, prefixes plugins.CommandPrefixes, commands ...plugins.Command) plugins.Snapshot {
		return plugins.Snapshot{PluginID: id, Valid: true, RegistrationState: "installed", DesiredState: "enabled", Commands: commands, CommandPrefixes: prefixes}
	}
	catalog := protocolPolicyCatalog{
		enabled("genshin", plugins.CommandPrefixes{}, plugins.Command{Name: "体力", Permission: "super_admin"}),
		enabled("starrail", plugins.CommandPrefixes{Dedicated: []string{"星铁", "*"}, IgnoreGlobal: true},
			plugins.Command{Name: "体力"}, plugins.Command{Name: "帮助"}),
	}
	current := func() config.Config {
		return config.Config{Command: &config.CommandConfig{Prefixes: []string{"/", "*"}}}
	}
	menu := menuext.New(menuext.Deps{CurrentConfig: current, Plugins: catalog})
	service := New(Deps{CurrentConfig: current, Plugins: catalog, Menu: menu})
	message := func(text string) chatevent.NormalizedEvent {
		return chatevent.NormalizedEvent{Kind: chatevent.EventKindMessage, SourceProtocol: "onebot11", SourceAdapter: "onebot11", EventType: "message.group", SenderID: "ordinary-user", ActorRole: "member", ConversationType: "group", ConversationID: "group-fixture", PlainText: text}
	}

	t.Run("dedicated prefix addresses only its plugin and uses that plugin's permission", func(t *testing.T) {
		enriched, allowed := service.Apply(context.Background(), message("*体力 100000001"))
		if !allowed {
			t.Fatal("the shadowed plugin's stricter permission rejected the command")
		}
		want := []chatevent.CommandTarget{{PluginID: "starrail", Command: "体力", Args: []string{"100000001"}}}
		if !enriched.CommandResolved || !reflect.DeepEqual(enriched.CommandTargets, want) {
			t.Fatalf("targets = %+v resolved=%v", enriched.CommandTargets, enriched.CommandResolved)
		}
	})
	t.Run("global prefix keeps addressing the plugin that accepts it", func(t *testing.T) {
		enriched, allowed := service.Apply(context.Background(), message("/体力"))
		if allowed {
			t.Fatal("super_admin command was allowed for an ordinary member")
		}
		if len(enriched.CommandTargets) != 1 || enriched.CommandTargets[0].PluginID != "genshin" {
			t.Fatalf("targets = %+v", enriched.CommandTargets)
		}
	})
	t.Run("dedicated prefix after the global prefix", func(t *testing.T) {
		enriched := service.EnrichCommandEvent(message("/星铁体力"))
		if len(enriched.CommandTargets) != 1 || enriched.CommandTargets[0].PluginID != "starrail" || enriched.PayloadFields["command"] != "体力" {
			t.Fatalf("targets = %+v payload = %v", enriched.CommandTargets, enriched.PayloadFields)
		}
	})
	t.Run("plugin help reached through a dedicated prefix shadows the builtin menu", func(t *testing.T) {
		if menu.Match(message("*帮助")).Matched {
			t.Fatal("builtin menu claimed a command addressed to a plugin")
		}
		if !menu.Match(message("/帮助")).Matched {
			t.Fatal("builtin menu no longer answers the global prefix")
		}
	})
	t.Run("mentions take no part in the command", func(t *testing.T) {
		mention := func(segments ...chatevent.MessageSegment) chatevent.NormalizedEvent {
			event := message(chatevent.PlainText(segments))
			event.Segments = segments
			return event
		}
		at := chatevent.MessageSegment{Type: "at", Data: map[string]any{"user_id": "10001"}}
		text := func(s string) chatevent.MessageSegment {
			return chatevent.MessageSegment{Type: "text", Data: map[string]any{"text": s}}
		}
		for name, event := range map[string]chatevent.NormalizedEvent{
			"after the words":  mention(text("*体力"), at),
			"before the words": mention(at, text(" *体力")),
		} {
			targets := service.EnrichCommandEvent(event).CommandTargets
			if len(targets) != 1 || targets[0].PluginID != "starrail" || targets[0].Command != "体力" || len(targets[0].Args) != 0 {
				t.Fatalf("%s: targets = %+v", name, targets)
			}
		}
		if !menu.Match(mention(at, text(" /帮助"))).Matched {
			t.Fatal("a mention before the builtin menu hid it")
		}
	})
	t.Run("a resolved command nobody declares has no targets", func(t *testing.T) {
		enriched := service.EnrichCommandEvent(message("/unknown"))
		if !enriched.CommandResolved || len(enriched.CommandTargets) != 0 || enriched.PayloadFields["command"] != "unknown" {
			t.Fatalf("resolved=%v targets=%+v payload=%v", enriched.CommandResolved, enriched.CommandTargets, enriched.PayloadFields)
		}
	})
}
