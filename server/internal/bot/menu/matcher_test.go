package menu

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func menuConfig(prefix string) config.Config {
	return config.Config{Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Prefixes: []string{prefix}, Commands: []string{"menu"}}}}
}

func TestMatchUsesTheMatcherRebuiltByUpdateConfig(t *testing.T) {
	t.Parallel()

	cfg := menuConfig("!")
	service := New(Deps{CurrentConfig: func() config.Config { return cfg }})

	if request := service.Match(chatevent.NormalizedEvent{PlainText: "!menu weather"}); !request.Matched || request.Target != "weather" || request.Prefix != "!" {
		t.Fatalf("expected the initial prefix to match, got %+v", request)
	}

	service.UpdateConfig(menuConfig("#"))

	if request := service.Match(chatevent.NormalizedEvent{PlainText: "!menu"}); request.Matched {
		t.Fatalf("old prefix must stop matching after UpdateConfig, got %+v", request)
	}
	if request := service.Match(chatevent.NormalizedEvent{PlainText: "#menu"}); !request.Matched || request.Prefix != "#" {
		t.Fatalf("new prefix must match after UpdateConfig, got %+v", request)
	}
}

func TestPluginMenuWordsBeatPluginPatterns(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Prefixes: []string{"/"}, Commands: []string{"帮助"}}}}
	snapshot := func(id, name string, commands ...plugins.Command) plugins.Snapshot {
		return plugins.Snapshot{PluginID: id, Name: name, Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running", Commands: commands}
	}
	service := New(Deps{CurrentConfig: func() config.Config { return cfg }, Plugins: plugincatalog.New([]plugins.Snapshot{
		snapshot("raylea.mihoyo-accounts", "米游社账号",
			plugins.Command{ID: "login", Name: "扫码登录", TriggerType: "exact", TriggerNames: []string{"扫码登录"}, Permission: "everyone"}),
		snapshot("raylea.genshin", "原神",
			plugins.Command{ID: "search", Name: "米游社搜索", TriggerType: "pattern", MatchPattern: `^(?:搜索|(?:米游社|mys)(?P<keyword>.+))$`, Permission: "everyone"},
			plugins.Command{ID: "gacha-help", Name: "抽卡帮助", TriggerType: "exact", TriggerNames: []string{"抽卡帮助"}, Permission: "everyone"},
			plugins.Command{ID: "pay-help", Name: "充值记录帮助", TriggerType: "pattern", MatchPattern: `^(?:充值|消费)(?:记录|统计)帮助$`, Permission: "everyone"}),
	})})
	event := func(text string) chatevent.NormalizedEvent {
		return chatevent.NormalizedEvent{PlainText: text, ConversationType: "private", ConversationID: "10002", SenderID: "10002"}
	}

	// A plugin's name before 帮助 opens its menu page though a pattern of
	// another plugin also takes the word.
	for text, target := range map[string]string{"/米游社账号帮助": "米游社账号", "/原神帮助": "原神"} {
		if request := service.Match(event(text)); !request.Matched || request.Target != target {
			t.Errorf("%s = %+v", text, request)
		}
	}
	// An exact plugin word, and a word naming no plugin, stay with the plugins.
	for _, text := range []string{"/抽卡帮助", "/充值记录帮助"} {
		if request := service.Match(event(text)); request.Matched {
			t.Errorf("%s was taken by the menu: %+v", text, request)
		}
	}
}
