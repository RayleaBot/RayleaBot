package runtime

import (
	"slices"
	"testing"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestRetainConfigFieldsKeepsCurrentValuesAtDocumentPaths(t *testing.T) {
	t.Parallel()

	current := internalconfig.Config{
		Server: internalconfig.ServerConfig{Host: "127.0.0.1", Port: 8080},
		Render: internalconfig.RenderConfig{BrowserArgs: []string{"--disable-gpu"}},
		Adapters: []internalconfig.AdapterInstance{
			{ID: "qq", Type: internalconfig.AdapterTypeQQOfficial, QQOfficial: &internalconfig.QQOfficialConfig{AppID: "1", Intents: []string{"GUILDS"}}},
			{ID: "onebot", Type: internalconfig.AdapterTypeOneBot11},
		},
	}
	desired := internalconfig.Config{
		Server:  internalconfig.ServerConfig{Host: "0.0.0.0", Port: 9090},
		Render:  internalconfig.RenderConfig{BrowserArgs: []string{"--headless=new"}},
		Command: &internalconfig.CommandConfig{Prefixes: []string{"/"}},
		Adapters: []internalconfig.AdapterInstance{
			{ID: "onebot", Type: internalconfig.AdapterTypeOneBot11, OneBot11: &internalconfig.OneBotConfig{}},
			{ID: "qq", Type: internalconfig.AdapterTypeQQOfficial, QQOfficial: &internalconfig.QQOfficialConfig{AppID: "2", Intents: []string{"GROUP"}}},
			{ID: "added", Type: internalconfig.AdapterTypeOneBot11},
		},
	}

	effective := retainConfigFields(current, desired, []string{
		"server.port",
		"render.browser_args",
		"command",
		"adapters.qq.qqofficial.intents",
		"adapters.onebot.onebot11",
		"adapters.removed.enabled",
	})

	if effective.Server.Port != 8080 || effective.Server.Host != "0.0.0.0" {
		t.Fatalf("server = %+v, want the current port with the desired host", effective.Server)
	}
	if !slices.Equal(effective.Render.BrowserArgs, []string{"--disable-gpu"}) {
		t.Fatalf("browser args = %#v, want the current arguments", effective.Render.BrowserArgs)
	}
	// An unset command section is part of the current document, so keeping the
	// path keeps it unset.
	if effective.Command != nil {
		t.Fatalf("command = %#v, want the current unset section", effective.Command)
	}
	qq := effective.Adapters[1].QQOfficial
	if qq.AppID != "2" || !slices.Equal(qq.Intents, []string{"GUILDS"}) {
		t.Fatalf("qq settings = %+v, want the desired app id with the current intents", qq)
	}
	// The current entry omits its onebot11 section, so there is no value to keep.
	if effective.Adapters[0].OneBot11 == nil {
		t.Fatal("desired onebot11 section was cleared although the current entry omits it")
	}
	if len(effective.Adapters) != 3 {
		t.Fatalf("adapters = %#v, want the desired entries", effective.Adapters)
	}

	current.Render.BrowserArgs[0] = "changed"
	current.Adapters[0].QQOfficial.Intents[0] = "changed"
	if effective.Render.BrowserArgs[0] != "--disable-gpu" || qq.Intents[0] != "GUILDS" {
		t.Fatal("retained values share storage with the current configuration")
	}
}

func TestRetainConfigFieldsDoesNotCreateAdapterEntries(t *testing.T) {
	t.Parallel()

	current := internalconfig.Config{Adapters: []internalconfig.AdapterInstance{{ID: "qq", Type: internalconfig.AdapterTypeQQOfficial, Enabled: true}}}
	effective := retainConfigFields(current, internalconfig.Config{}, []string{"adapters.qq.enabled"})
	if effective.Adapters != nil {
		t.Fatalf("adapters = %#v, want no entry created in a configuration without adapters", effective.Adapters)
	}
}
