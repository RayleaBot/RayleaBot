package chatpolicy

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

// A plugin that draws its own help answers the builtin menu's page for it;
// one that does not keeps the builtin page.
func TestBuiltinMenuHandsPluginPageToItsHelpCommand(t *testing.T) {
	cfg := config.Config{Command: &config.CommandConfig{Prefixes: []string{"/"}}, Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Prefixes: []string{"/"}, Commands: []string{"help", "帮助"}}}}
	current := func() config.Config { return cfg }
	help := plugins.Command{ID: "help", Name: "帮助", TriggerType: "exact", TriggerNames: []string{"帮助", "help"}, Permission: "everyone"}
	catalog := protocolPolicyCatalog{
		{PluginID: "raylea.starrail", Name: "崩坏：星穹铁道", Valid: true, RegistrationState: "installed", DesiredState: "enabled", Commands: []plugins.Command{help}, Help: &plugins.Help{Title: "崩坏：星穹铁道", Command: "help"}, CommandPrefixes: plugins.CommandPrefixes{Dedicated: []string{"*"}, IgnoreGlobal: true}},
		{PluginID: "echo", Name: "回声", Valid: true, RegistrationState: "installed", DesiredState: "enabled", Commands: []plugins.Command{{ID: "echo", Name: "echo", TriggerType: "exact", TriggerNames: []string{"echo"}, Permission: "everyone"}}, Help: &plugins.Help{Title: "回声"}},
	}
	builtin := menu.New(menu.Deps{CurrentConfig: current, Plugins: catalog})
	service := New(Deps{CurrentConfig: current, Plugins: catalog, Menu: builtin})

	for _, text := range []string{"/崩坏：星穹铁道帮助", "/帮助 崩坏：星穹铁道"} {
		event := service.EnrichCommandEvent(chatevent.NormalizedEvent{PlainText: text})
		if len(event.CommandTargets) != 1 || event.CommandTargets[0].PluginID != "raylea.starrail" || event.CommandTargets[0].Command != "帮助" {
			t.Fatalf("%s: %+v", text, event.CommandTargets)
		}
		if builtin.Match(chatevent.NormalizedEvent{PlainText: text}).Delegate == nil {
			t.Fatalf("%s: the menu would draw the page itself", text)
		}
	}
	if request := builtin.Match(chatevent.NormalizedEvent{PlainText: "/回声帮助"}); !request.Matched || request.Delegate != nil {
		t.Fatalf("a plugin without its own help lost the builtin page: %+v", request)
	}
}
