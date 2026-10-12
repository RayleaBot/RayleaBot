package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
)

func TestConfigUpdateRemovesObsoleteSecretsAfterPersistence(t *testing.T) {
	for _, mode := range []string{"delete instances", "rename instances", "save failure"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()
			path := filepath.Join(root, "user.yaml")
			cfg, _, err := config.Init(path, "")
			if err != nil {
				t.Fatal(err)
			}
			cfg.Adapters = []config.AdapterInstance{
				{ID: "onebot", Type: config.AdapterTypeOneBot11, OneBot11: &config.OneBotConfig{
					ForwardWS: config.OneBotTransportConfig{AccessToken: "fixture-forward"},
					ReverseWS: config.OneBotTransportConfig{AccessToken: "fixture-reverse"},
					HTTPAPI:   config.OneBotTransportConfig{AccessToken: "fixture-http"},
					Webhook:   config.OneBotTransportConfig{AccessToken: "fixture-webhook"},
				}},
				{ID: "qq", Type: config.AdapterTypeQQOfficial, QQOfficial: &config.QQOfficialConfig{
					AppID: "123456", AppSecret: "fixture-app-secret",
				}},
			}
			store := newMemorySecretStore()
			stored, err := StoreConfigSecrets(ctx, store, ConfigDocumentFromTyped(cfg))
			if err != nil {
				t.Fatal(err)
			}
			_, summary, err := config.SaveDocument(path, "", stored)
			if err != nil {
				t.Fatal(err)
			}
			oldKeys, err := store.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			request := ConfigDocumentFromTyped(cfg)
			if mode == "rename instances" {
				for _, entry := range request["adapters"].([]any) {
					adapter := entry.(map[string]any)
					adapter["id"] = adapter["id"].(string) + "-renamed"
				}
			} else {
				request["adapters"] = []any{}
			}
			if mode == "save failure" {
				occupied := filepath.Join(root, "occupied")
				if err := os.WriteFile(occupied, []byte("fixture"), 0o600); err != nil {
					t.Fatal(err)
				}
				summary.ConfigPath = filepath.Join(occupied, "user.yaml")
			}
			service := NewService(Deps{CurrentConfig: func() config.Config { return cfg },
				CurrentSummary: func() config.Summary { return summary }, Secrets: store})
			_, err = service.UpdateConfigDocument(ctx, request)
			if mode == "save failure" {
				var persistenceError *PersistenceError
				if !errors.As(err, &persistenceError) {
					t.Fatalf("expected persistence failure, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			for _, key := range oldKeys {
				if _, err := store.Get(ctx, key); mode == "save failure" {
					if err != nil {
						t.Fatalf("failed save removed %s: %v", key, err)
					}
				} else if !errors.Is(err, secrets.ErrNotFound) {
					t.Fatalf("obsolete key %s remains: %v", key, err)
				}
			}
			saved, _, err := config.Load(path, "")
			if err != nil {
				t.Fatal(err)
			}
			resolved, err := ResolveConfigSecretRefs(ctx, store, saved)
			if err != nil {
				t.Fatalf("saved config has unresolved secrets: %v", err)
			}
			if mode == "delete instances" {
				if len(resolved.Adapters) != 0 {
					t.Fatal("deleted instances remain in the saved config")
				}
			} else if resolved.Adapters[0].OneBot11.ForwardWS.AccessToken != "fixture-forward" ||
				resolved.Adapters[1].QQOfficial.AppSecret != "fixture-app-secret" {
				t.Fatal("saved config lost credentials")
			}
		})
	}
}

func TestFailedConfigPersistenceRestoresCredentialsAndRevision(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "user.yaml")
	cfg, summary, err := config.Init(path, "")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Adapters = []config.AdapterInstance{{ID: "test", Type: config.AdapterTypeOneBot11, OneBot11: &config.OneBotConfig{}}}
	store := newMemorySecretStore()
	oldDocument := ConfigDocumentFromTyped(cfg)
	secretPath := onebotSecretPath("test", "forward_ws")
	setConfigPath(oldDocument, secretPath, "old-fixture-token")
	stored, err := StoreConfigSecrets(ctx, store, oldDocument)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err = config.SaveDocument(path, "", stored)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = ResolveConfigSecretRefs(ctx, store, cfg)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	// A file occupying the parent path makes replacement fail on every OS.
	occupied := filepath.Join(t.TempDir(), "occupied")
	if err := os.WriteFile(occupied, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	summary.ConfigPath = filepath.Join(occupied, "user.yaml")
	service := NewService(Deps{CurrentConfig: func() config.Config { return cfg },
		CurrentSummary: func() config.Summary { return summary }, Secrets: store})
	request := ConfigDocumentFromTyped(cfg)
	setConfigPath(request, secretPath, "new-fixture-token")
	if _, err := service.UpdateConfigDocument(ctx, request); err == nil {
		t.Fatal("expected persistence failure")
	}
	value, err := store.Get(ctx, configSecretKey(secretPath))
	if err != nil || string(value) != "old-fixture-token" {
		t.Fatalf("credential was not restored: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) || service.CurrentConfigDocument().Revision != 1 {
		t.Fatal("failed update changed persisted config or revision")
	}
}

func TestConfigPersistenceRevalidatesChangedExternalSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user.yaml")
	cfg, summary, err := config.Init(path, "")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Adapters = []config.AdapterInstance{{ID: "test", Type: config.AdapterTypeOneBot11, OneBot11: &config.OneBotConfig{}}}
	schemaPath := filepath.Join(t.TempDir(), "config.schema.json")
	if err := os.WriteFile(schemaPath, []byte("true"), 0o600); err != nil {
		t.Fatal(err)
	}
	summary.SchemaPath = schemaPath
	store := &schemaChangingSecretStore{
		memorySecretStore: newMemorySecretStore(),
		schemaPath:        schemaPath,
	}
	service := NewService(Deps{CurrentConfig: func() config.Config { return cfg },
		CurrentSummary: func() config.Summary { return summary }, Secrets: store})
	request := ConfigDocumentFromTyped(cfg)
	secretPath := onebotSecretPath("test", "forward_ws")
	setConfigPath(request, secretPath, "fixture-token")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateConfigDocument(context.Background(), request); err == nil {
		t.Fatal("persisted a document rejected by the replacement external schema")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !store.changed || string(before) != string(after) || service.CurrentConfigDocument().Revision != 1 || len(store.values) != 0 {
		t.Fatal("failed second validation did not preserve file, secrets and revision")
	}
}

type schemaChangingSecretStore struct {
	*memorySecretStore
	schemaPath string
	changed    bool
}

func (s *schemaChangingSecretStore) Set(ctx context.Context, key string, value []byte) error {
	if err := s.memorySecretStore.Set(ctx, key, value); err != nil {
		return err
	}
	if !s.changed {
		s.changed = true
		return os.WriteFile(s.schemaPath, []byte("false"), 0o600)
	}
	return nil
}

type reloadFailure struct{ calls int }

func (r *reloadFailure) ApplyConfigReload(config.Config) error {
	r.calls++
	return errors.New("fixture adapter failed")
}

func (*reloadFailure) PublishSnapshot() {}

func TestReloadFailureKeepsDesiredFileAndReportsPendingGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user.yaml")
	cfg, summary, err := config.Init(path, "")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Adapters = []config.AdapterInstance{{ID: "test", Type: config.AdapterTypeOneBot11, OneBot11: &config.OneBotConfig{}}}
	protocol := &reloadFailure{}
	service := NewService(Deps{CurrentConfig: func() config.Config { return cfg },
		CurrentSummary: func() config.Summary { return summary }, SetConfig: func(next config.Config) { cfg = next }, Protocol: protocol})
	request := ConfigDocumentFromTyped(cfg)
	setConfigPath(request, []string{"adapters", "test", "onebot11", "forward_ws", "url"}, "ws://127.0.0.1:9001")
	result, err := service.UpdateConfigDocument(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.RestartRequired || result.Document.Revision != 2 || len(result.ApplyEffects.FailedGroups) != 1 || result.ApplyEffects.FailedGroups[0] != "adapters" {
		t.Fatalf("missing saved but unapplied result: %+v", result)
	}
	if protocol.calls != 2 || cfg.Adapters[0].OneBot11.ForwardWS.URL != "" {
		t.Fatal("adapter group did not retain its previous effective settings and attempt restoration")
	}
	saved, _, err := config.Load(path, "")
	if err != nil || saved.Adapters[0].OneBot11.ForwardWS.URL != "ws://127.0.0.1:9001" {
		t.Fatalf("desired settings were not saved: %v", err)
	}
}

func TestConcurrentConfigSnapshotsContainOneRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user.yaml")
	cfg, summary, err := config.Init(path, "")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(Deps{CurrentConfig: func() config.Config { return cfg },
		CurrentSummary: func() config.Summary { return summary }, SetConfig: func(next config.Config) { cfg = next }})
	var readers sync.WaitGroup
	stop := make(chan struct{})
	for range 4 {
		readers.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				snapshot := service.CurrentConfigDocument()
				if snapshot.Revision > 1 {
					port := snapshot.Config["server"].(map[string]any)["port"].(float64)
					if port != float64(10000+snapshot.Revision) {
						t.Errorf("mixed config snapshot: revision=%d port=%v", snapshot.Revision, port)
						return
					}
				}
			}
		})
	}
	for revision := uint64(2); revision <= 10; revision++ {
		request := service.CurrentConfigDocument().Config
		request["server"].(map[string]any)["port"] = 10000 + revision
		if _, err := service.UpdateConfigDocument(context.Background(), request); err != nil {
			t.Error(err)
			break
		}
	}
	close(stop)
	readers.Wait()
	if cfg.Server.Port != 8080 {
		t.Fatal("restart-only field changed in the running configuration")
	}
}
