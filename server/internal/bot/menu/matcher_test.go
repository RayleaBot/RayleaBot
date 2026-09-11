package menu

import (
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
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
