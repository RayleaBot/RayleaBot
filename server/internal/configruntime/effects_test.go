package configruntime

import (
	"path/filepath"
	"slices"
	"testing"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestConfigSchemaMetadataCoversCanonicalFields(t *testing.T) {
	t.Parallel()

	cfg, _, err := internalconfig.Load(filepath.Join(t.TempDir(), "config", "user.yaml"), "")
	if err != nil {
		t.Fatalf("load default config: %v", err)
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
	for _, path := range ConfigFieldMetadataPaths() {
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
		"onebot.forward_ws.access_token",
		"onebot.http_api.access_token",
		"onebot.reverse_ws.access_token",
		"onebot.webhook.access_token",
		"qq_official.app_secret",
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
	return paths
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
	if prefix != "" {
		*paths = append(*paths, prefix)
	}
}
