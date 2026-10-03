package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/secrets"
	secretsqlite "github.com/RayleaBot/RayleaBot/server/internal/platform/secrets/sqlite"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type configSQLiteBatchHook struct {
	*secretsqlite.Store
	afterApply func() error
	applied    bool
}

func (s *configSQLiteBatchHook) Apply(ctx context.Context, values map[string][]byte, deleted []string) error {
	if err := s.Store.Apply(ctx, values, deleted); err != nil {
		return err
	}
	s.applied = true
	if s.afterApply != nil {
		return s.afterApply()
	}
	return nil
}

func TestConfigSQLiteBatchFailurePreservesCredentialsFileAndRevision(t *testing.T) {
	for _, mode := range []string{"file replacement", "external schema", "cancelled after commit"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			root := t.TempDir()
			path := filepath.Join(root, "user.yaml")
			cfg, _, err := config.Init(path, "")
			if err != nil {
				t.Fatal(err)
			}
			cfg.Adapters = []config.AdapterInstance{{ID: "fixture", Type: config.AdapterTypeOneBot11,
				OneBot11: &config.OneBotConfig{ForwardWS: config.OneBotTransportConfig{AccessToken: "fixture-old"}}}}
			db, err := storage.Open(filepath.Join(root, "secrets.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			base, err := secretsqlite.NewStore(db)
			if err != nil {
				t.Fatal(err)
			}
			store := &configSQLiteBatchHook{Store: base}
			if err := store.Set(ctx, "plugin:fixture:secret:token", []byte("unrelated")); err != nil {
				t.Fatal(err)
			}
			stored, err := StoreConfigSecrets(ctx, store, ConfigDocumentFromTyped(cfg))
			if err != nil {
				t.Fatal(err)
			}
			_, summary, err := config.SaveDocument(path, "", stored)
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "file replacement":
				occupied := filepath.Join(root, "occupied")
				if err := os.WriteFile(occupied, []byte("fixture"), 0o600); err != nil {
					t.Fatal(err)
				}
				summary.ConfigPath = filepath.Join(occupied, "user.yaml")
			case "external schema":
				schemaPath := filepath.Join(root, "config.schema.json")
				if err := os.WriteFile(schemaPath, []byte("true"), 0o600); err != nil {
					t.Fatal(err)
				}
				summary.SchemaPath = schemaPath
				store.afterApply = func() error { return os.WriteFile(schemaPath, []byte("false"), 0o600) }
			case "cancelled after commit":
				store.afterApply = func() error { cancel(); return nil }
			}
			published := false
			service := NewService(Deps{CurrentConfig: func() config.Config { return cfg },
				CurrentSummary: func() config.Summary { return summary },
				SetConfig:      func(config.Config) { published = true }, Secrets: store})
			request := ConfigDocumentFromTyped(cfg)
			existingPath := onebotSecretPath("fixture", "forward_ws")
			createdPath := onebotSecretPath("fixture", "reverse_ws")
			setConfigPath(request, existingPath, "fixture-new")
			setConfigPath(request, createdPath, "fixture-created")
			_, err = service.UpdateConfigDocument(ctx, request)
			var persistenceError *PersistenceError
			if !errors.As(err, &persistenceError) || !store.applied || published || service.CurrentConfigDocument().Revision != 1 {
				t.Fatalf("failed batch publication result: %v", err)
			}
			if mode == "cancelled after commit" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation error: %v", err)
			}
			for key, want := range map[string]string{configSecretKey(existingPath): "fixture-old", "plugin:fixture:secret:token": "unrelated"} {
				got, err := store.Get(context.Background(), key)
				if err != nil || string(got) != want {
					t.Fatalf("credential was changed by failed config update: %v", err)
				}
			}
			if _, err := store.Get(context.Background(), configSecretKey(createdPath)); !errors.Is(err, secrets.ErrNotFound) {
				t.Fatalf("failed update retained a newly created credential: %v", err)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(before) != string(after) {
				t.Fatalf("failed update changed the previous config file: %v", err)
			}
		})
	}
}
