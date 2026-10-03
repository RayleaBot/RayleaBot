package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
)

func TestUpdateConfigDocumentUsesRequestContextForSecrets(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	cfg, summary, err := internalconfig.Init(configPath, "")
	if err != nil {
		t.Fatalf("init config: %v", err)
	}
	cfg.Adapters = []internalconfig.AdapterInstance{{
		ID:       internalconfig.DefaultOneBot11AdapterID,
		Type:     internalconfig.AdapterTypeOneBot11,
		OneBot11: &internalconfig.OneBotConfig{},
	}}
	request := ConfigDocumentFromTyped(cfg)
	setConfigPath(request, onebotSecretPath(internalconfig.DefaultOneBot11AdapterID, "forward_ws"), "forward-secret")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := NewService(Deps{
		CurrentConfig: func() internalconfig.Config { return cfg },
		CurrentSummary: func() internalconfig.Summary {
			return summary
		},
		Secrets: contextCheckingSecretStore{},
	})

	_, err = service.UpdateConfigDocument(ctx, request)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateConfigDocument error = %v, want context.Canceled", err)
	}
}

type contextCheckingSecretStore struct{}

func TestTimezoneChangeWaitsForRestartWithoutChangingEffectiveTimezone(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config", "user.yaml")
	cfg, summary, err := internalconfig.Init(configPath, "")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(Deps{
		CurrentConfig:     func() internalconfig.Config { return cfg },
		CurrentSummary:    func() internalconfig.Summary { return summary },
		SetConfig:         func(next internalconfig.Config) { cfg = next },
		EffectiveTimezone: func() string { return "Asia/Shanghai" },
		Secrets:           contextCheckingSecretStore{},
	})
	request := ConfigDocumentFromTyped(cfg)
	request["scheduler"].(map[string]any)["timezone"] = "America/New_York"
	result, err := service.UpdateConfigDocument(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RestartRequired || result.Document.EffectiveTimezone != "Asia/Shanghai" {
		t.Fatalf("wrong timezone apply result: %+v", result)
	}
	if got := service.CurrentConfigDocument(); got.EffectiveTimezone != "Asia/Shanghai" || got.Config["scheduler"].(map[string]any)["timezone"] != "America/New_York" {
		t.Fatalf("wrong persisted/effective timezone: %+v", got)
	}
}

func TestConcurrentConfigDocumentsOwnTheirRedactedValues(t *testing.T) {
	cfg := internalconfig.Config{Adapters: []internalconfig.AdapterInstance{{
		ID: "qq", Type: internalconfig.AdapterTypeQQOfficial,
		QQOfficial: &internalconfig.QQOfficialConfig{AppSecret: "fixture-secret", Intents: []string{"group_and_c2c"}},
	}}}
	service := NewService(Deps{CurrentConfig: func() internalconfig.Config { return cfg }})
	var readers sync.WaitGroup
	for range 8 {
		readers.Go(func() {
			for range 10 {
				snapshot := service.CurrentConfigDocument()
				settings := snapshot.Config["adapters"].([]any)[0].(map[string]any)["qqofficial"].(map[string]any)
				if settings["app_secret"] != redactedConfigValue || settings["intents"].([]any)[0] != "group_and_c2c" || len(snapshot.RedactedFields) != 1 || snapshot.RedactedFields[0] != "adapters.qq.qqofficial.app_secret" {
					t.Error("config snapshot exposed a secret or another caller's mutation")
					return
				}
				settings["intents"].([]any)[0] = "changed"
				settings["app_secret"] = "changed"
				snapshot.RedactedFields[0] = "changed"
			}
		})
	}
	readers.Wait()
	if cfg.Adapters[0].QQOfficial.AppSecret != "fixture-secret" || cfg.Adapters[0].QQOfficial.Intents[0] != "group_and_c2c" {
		t.Fatal("config snapshot mutated the source")
	}
}

func (contextCheckingSecretStore) Get(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, secrets.ErrNotFound
}

func (contextCheckingSecretStore) Set(ctx context.Context, _ string, _ []byte) error {
	return ctx.Err()
}

func (contextCheckingSecretStore) Delete(ctx context.Context, _ string) error {
	return ctx.Err()
}

func (contextCheckingSecretStore) List(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}
