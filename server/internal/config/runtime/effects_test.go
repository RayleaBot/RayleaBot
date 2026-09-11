package runtime

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestReferencedFieldRetainsItsApplyPolicyOverride(t *testing.T) {
	t.Parallel()
	current, _, err := internalconfig.Load(filepath.Join(t.TempDir(), "config", "user.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	next := current
	next.Runtime.IPCActionBurstLimit = "180/5s"
	effects := ClassifyApplyEffects(current, next)
	if !slices.Equal(effects.RestartRequiredFields, []string{"runtime.ipc_action_burst_limit"}) || len(effects.AppliedNow) != 0 {
		t.Fatalf("field-specific policy was replaced by the rateLimit definition: %#v", effects)
	}
}

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

func TestDouyinLoginBrowserSettingsRequireRestart(t *testing.T) {
	t.Parallel()

	current, _, err := internalconfig.Load(filepath.Join(t.TempDir(), "config", "user.yaml"), "")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	next := current
	next.ThirdParty.DouyinLogin.BrowserMode = "remote_cdp"
	next.ThirdParty.DouyinLogin.RemoteDebuggingURL = "http://127.0.0.1:9222"

	effects := ClassifyApplyEffects(current, next)
	want := []string{
		"third_party_accounts.douyin_login.browser_mode",
		"third_party_accounts.douyin_login.remote_debugging_url",
	}
	if !slices.Equal(effects.RestartRequiredFields, want) {
		t.Fatalf("restart-required fields = %#v, want %#v", effects.RestartRequiredFields, want)
	}
	if len(effects.AppliedNow) != 0 || len(effects.ReloadedNow) != 0 {
		t.Fatalf("unexpected immediate effects: %#v", effects)
	}
}

type accountValidationConfigRecorder struct {
	interval int
}

func (r *accountValidationConfigRecorder) ApplyConfig(cfg internalconfig.Config) {
	r.interval = cfg.ThirdParty.CredentialCheckIntervalMinutes
}

func TestCredentialCheckIntervalHotReloadsMonitor(t *testing.T) {
	t.Parallel()

	current, _, err := internalconfig.Load(filepath.Join(t.TempDir(), "config", "user.yaml"), "")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	next := current
	next.ThirdParty.CredentialCheckIntervalMinutes = 720
	recorder := &accountValidationConfigRecorder{}
	service := NewService(Deps{
		CurrentConfig:     func() internalconfig.Config { return current },
		SetConfig:         func(cfg internalconfig.Config) { current = cfg },
		AccountValidation: recorder,
	})

	effects := service.ApplyHotReloadableFields(next)

	if recorder.interval != 720 {
		t.Fatalf("credential monitor interval = %d, want 720", recorder.interval)
	}
	want := []string{"third_party_accounts.credential_check_interval_minutes"}
	if !slices.Equal(effects.AppliedNow, want) || len(effects.ReloadedNow) != 0 || len(effects.RestartRequiredFields) != 0 {
		t.Fatalf("unexpected apply effects: %#v", effects)
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
	next := current
	next.Render.QueueMaxLength = current.Render.QueueMaxLength + 1
	next.Message.CircuitBreakerSeconds = current.Message.CircuitBreakerSeconds + 1

	renderer := &configApplyCounter{}
	outbound := &configApplyCounter{}
	pluginLog := &configApplyCounter{}
	accountValidation := &configApplyCounter{}
	service := NewService(Deps{
		CurrentConfig:     func() internalconfig.Config { return current },
		SetConfig:         func(cfg internalconfig.Config) { current = cfg },
		Renderer:          renderer,
		OutboundLimiter:   outbound,
		PluginLogLimiter:  pluginLog,
		AccountValidation: accountValidation,
	})

	service.ApplyHotReloadableFields(next)

	if renderer.calls != 1 || outbound.calls != 1 {
		t.Fatalf("renderer=%d outbound=%d, want one apply each", renderer.calls, outbound.calls)
	}
	if pluginLog.calls != 0 || accountValidation.calls != 0 {
		t.Fatalf("plugin log=%d account validation=%d, want no apply", pluginLog.calls, accountValidation.calls)
	}
}
