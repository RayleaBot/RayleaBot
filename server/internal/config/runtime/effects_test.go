package runtime

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestMetadataRejectsRecursiveReferences(t *testing.T) {
	t.Parallel()
	for _, definition := range []string{
		`{"$ref":"#/$defs/loop"}`,
		`{"type":"object","properties":{"child":{"$ref":"#/$defs/loop"}}}`,
	} {
		_, err := loadConfigFieldMetadata([]byte(`{"type":"object","properties":{"item":{"$ref":"#/$defs/loop"}},"$defs":{"loop":` + definition + `}}`))
		if err == nil || !strings.Contains(err.Error(), "cyclic config schema reference") {
			t.Fatalf("recursive reference must fail with a bounded error, got %v", err)
		}
	}
}

func TestConfigSchemaMetadataCoversCanonicalFields(t *testing.T) {
	t.Parallel()

	cfg, _, err := internalconfig.Load(filepath.Join(t.TempDir(), "config", "user.yaml"), "")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	// A default install configures no adapters, so the document is given one
	// instance of each type: the parity checked here is between the schema and
	// the fields a document can hold, not the fields one install happens to set.
	cfg.Adapters = []internalconfig.AdapterInstance{
		{ID: "onebot11", Type: internalconfig.AdapterTypeOneBot11, OneBot11: &internalconfig.OneBotConfig{}},
		{ID: "qq-official", Type: internalconfig.AdapterTypeQQOfficial, QQOfficial: &internalconfig.QQOfficialConfig{}},
	}
	paths := collectConfigLeafPaths(ConfigDocumentFromTyped(cfg))

	var missing []string
	for _, path := range paths {
		if _, ok := ConfigFieldMetadataForPath(path); !ok {
			missing = append(missing, path)
		}
	}
	if len(missing) != 0 {
		t.Fatalf("missing config schema metadata: %#v", missing)
	}

	pathSet := make(map[string]bool, len(paths))
	for _, path := range paths {
		pathSet[path] = true
	}
	var extra []string
	for path := range configFieldMetadata {
		if !pathSet[path] {
			extra = append(extra, path)
		}
	}
	slices.Sort(extra)
	if len(extra) != 0 {
		t.Fatalf("config schema metadata for unknown fields: %#v", extra)
	}
}

func TestConfigSchemaMetadataMarksSecrets(t *testing.T) {
	t.Parallel()

	want := []string{
		"adapters.*.onebot11.forward_ws.access_token",
		"adapters.*.onebot11.http_api.access_token",
		"adapters.*.onebot11.reverse_ws.access_token",
		"adapters.*.onebot11.webhook.access_token",
		"adapters.*.qqofficial.app_secret",
	}
	got := ConfigSecretFieldPaths()
	if !slices.Equal(got, want) {
		t.Fatalf("secret config fields = %#v, want %#v", got, want)
	}
	for _, path := range got {
		metadata, ok := ConfigFieldMetadataForPath(path)
		if !ok {
			t.Fatalf("missing metadata for secret path %s", path)
		}
		if metadata.ApplyPolicy != ConfigApplyPolicySecretOnly || !metadata.Secret || metadata.Redaction != "full" {
			t.Fatalf("unexpected secret metadata for %s: %#v", path, metadata)
		}
	}
}

func collectConfigLeafPaths(document map[string]any) []string {
	var paths []string
	collectConfigLeafPath("", document, &paths)
	slices.Sort(paths)
	// Every entry of a collection resolves to the same shape.
	return slices.Compact(paths)
}

func collectConfigLeafPath(prefix string, value any, paths *[]string) {
	if object, ok := value.(map[string]any); ok {
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			collectConfigLeafPath(joinConfigPath(prefix, key), object[key], paths)
		}
		return
	}
	// A keyed collection contributes the shape its entries share, plus the
	// collection path itself, which carries the policy for membership changes.
	if entries, ok := value.([]any); ok && isConfigCollection(prefix) {
		*paths = append(*paths, prefix)
		for _, entry := range entries {
			collectConfigLeafPath(joinConfigPath(prefix, ConfigCollectionWildcard), entry, paths)
		}
		return
	}
	if prefix != "" {
		*paths = append(*paths, prefix)
	}
}

func isConfigCollection(prefix string) bool {
	_, ok := ConfigCollectionKey(ConfigShapePath(prefix))
	return ok
}

type configApplyCounter struct {
	calls int
}

func (c *configApplyCounter) ApplyConfig(internalconfig.Config) {
	c.calls++
}

func TestHotReloadConsumersOnlyReceiveOwnedChanges(t *testing.T) {
	t.Parallel()

	current, _, err := internalconfig.Load(filepath.Join(t.TempDir(), "config", "user.yaml"), "")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}

	renderer := &configApplyCounter{}
	outbound := &configApplyCounter{}
	service := NewService(Deps{
		CurrentConfig:   func() internalconfig.Config { return current },
		SetConfig:       func(cfg internalconfig.Config) { current = cfg },
		Renderer:        renderer,
		OutboundLimiter: outbound,
	})

	renderOnly := current
	renderOnly.Render.QueueMaxLength = current.Render.QueueMaxLength + 1
	service.ApplyHotReloadableFields(renderOnly)
	if renderer.calls != 1 || outbound.calls != 0 {
		t.Fatalf("after render change: renderer=%d outbound=%d, want only the renderer applied", renderer.calls, outbound.calls)
	}

	messageOnly := current
	messageOnly.Message.RateLimitPerTarget = "7/5s"
	service.ApplyHotReloadableFields(messageOnly)
	if renderer.calls != 1 || outbound.calls != 1 {
		t.Fatalf("after message change: renderer=%d outbound=%d, want only the outbound limiter applied", renderer.calls, outbound.calls)
	}
}

func TestAdapterCollectionChangesUseReloadPolicy(t *testing.T) {
	first := internalconfig.AdapterInstance{ID: "first", Type: "onebot11", Enabled: true, OneBot11: &internalconfig.OneBotConfig{}}
	second := first
	second.ID = "second"
	for _, tc := range []struct {
		name          string
		before, after []internalconfig.AdapterInstance
	}{
		{"add", nil, []internalconfig.AdapterInstance{first}},
		{"remove", []internalconfig.AdapterInstance{first}, nil},
		{"rename", []internalconfig.AdapterInstance{first}, []internalconfig.AdapterInstance{second}},
		{"retype", []internalconfig.AdapterInstance{first}, []internalconfig.AdapterInstance{{ID: "first", Type: "qqofficial", QQOfficial: &internalconfig.QQOfficialConfig{}}}},
		{"reorder", []internalconfig.AdapterInstance{first, second}, []internalconfig.AdapterInstance{second, first}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldCfg, nextCfg := internalconfig.Config{Adapters: tc.before}, internalconfig.Config{Adapters: tc.after}
			effects := ClassifyApplyEffects(oldCfg, nextCfg)
			if effects.RestartRequired() || !slices.Equal(effects.ReloadedNow, []string{"adapters"}) {
				t.Fatalf("effects=%+v", effects)
			}
			effective := oldCfg
			protocol := &reloadFailure{}
			service := NewService(Deps{CurrentConfig: func() internalconfig.Config { return effective }, SetConfig: func(cfg internalconfig.Config) { effective = cfg }, Protocol: protocol})
			effects = service.ApplyHotReloadableFields(nextCfg)
			if !slices.Equal(effects.FailedGroups, []string{"adapters"}) || !slices.Equal(effects.RestartRequiredFields, []string{"adapters"}) || len(effects.ReloadedNow) != 0 || protocol.calls != 2 {
				t.Fatalf("failed reload effects=%+v, calls=%d", effects, protocol.calls)
			}
			if !slices.EqualFunc(effective.Adapters, oldCfg.Adapters, func(a, b internalconfig.AdapterInstance) bool { return a.ID == b.ID && a.Type == b.Type }) {
				t.Fatal("failed reload did not retain the old collection")
			}
		})
	}
}
