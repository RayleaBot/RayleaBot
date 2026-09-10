package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/filelock"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestAppRunListenFailureReleasesDatabaseAndConfigLock(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	options, databasePath := shutdownTestOptions(t, listener.Addr().(*net.TCPAddr).Port)
	application, err := app.New(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = application.Close() })
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	err = application.Run(ctx)
	var listenErr *net.OpError
	if !errors.As(err, &listenErr) || listenErr.Op != "listen" || !strings.Contains(err.Error(), "listen on ") {
		t.Fatalf("Run() = %v, want identified listener failure", err)
	}
	if repeated := application.Close(); !errors.Is(repeated, listenErr) {
		t.Fatalf("second Close() = %v, want original failure", repeated)
	}
	assertShutdownReleasesPersistentResources(t, options.ConfigPath, databasePath)
}

func TestAppConstructionFailureReleasesDatabaseAndConfigLock(t *testing.T) {
	options, databasePath := shutdownTestOptions(t, 8080)
	expected := errors.New("injected log retention failure")
	options.LogRepository = &failingRetentionRepository{err: expected}
	if _, err := app.New(options); !errors.Is(err, expected) {
		t.Fatalf("New() = %v, want retention failure", err)
	}
	assertShutdownReleasesPersistentResources(t, options.ConfigPath, databasePath)
}

func shutdownTestOptions(t *testing.T, port int) (app.Options, string) {
	t.Helper()
	fixture := testutil.LoadConfigFixture(t, testutil.RepoPath(t, "fixtures", "config", "ok.minimal.json"))
	var input map[string]any
	if err := json.Unmarshal(fixture.Input, &input); err != nil {
		t.Fatal(err)
	}
	input["server"].(map[string]any)["port"] = port
	databasePath := filepath.Join(t.TempDir(), "state.db")
	input["database"].(map[string]any)["path"] = databasePath
	input["log"].(map[string]any)["retention_days"] = 7
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	repoRoot := testutil.NewPreparedTestRuntimeRoot(t)
	return app.Options{
		ConfigPath:       testutil.WriteYAMLConfig(t, encoded),
		SchemaPath:       testutil.RepoPath(t, "contracts", "config.user.schema.json"),
		PluginRepoRoot:   repoRoot,
		PluginSchemaPath: testutil.RepoPath(t, "contracts", "plugin-info.schema.json"),
		PluginRoots:      []plugincatalog.ScanRoot{{Label: "plugins/installed", Path: filepath.Join(repoRoot, "plugins", "installed")}},
		RenderRunner:     shutdownTestRunner{},
	}, databasePath
}

func assertShutdownReleasesPersistentResources(t *testing.T, configPath, databasePath string) {
	t.Helper()
	store, err := storage.Open(databasePath)
	if err != nil {
		t.Fatalf("database remained locked after shutdown: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	lockPath, err := runtimepaths.ResolveConfigLifecycleLockPath(configPath)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("config remained locked after shutdown: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}

type shutdownTestRunner struct{}

func (shutdownTestRunner) Render(context.Context, renderservice.Document) ([]byte, error) {
	return nil, errors.New("unexpected render")
}

type failingRetentionRepository struct {
	logging.Repository
	err error
}

func (repository *failingRetentionRepository) PruneOlderThan(context.Context, time.Time) error {
	return repository.err
}
