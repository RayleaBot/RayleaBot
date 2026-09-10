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
	sealed, err := store.Get(ctx, configSecretKey(secretPath))
	if err != nil {
		t.Fatal(err)
	}
	value, err := secrets.OpenString(ctx, store, sealed)
	if err != nil || value != "old-fixture-token" {
		t.Fatalf("credential was not restored: %v", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) || service.CurrentConfigDocument().Revision != 1 {
		t.Fatal("failed update changed persisted config or revision")
	}
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
