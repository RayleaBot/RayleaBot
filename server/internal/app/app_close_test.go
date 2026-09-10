package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/filelock"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
	"github.com/RayleaBot/RayleaBot/server/internal/runtimepaths"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

type runtimeBoundDelivery struct {
	manager *pluginruntime.Manager
	started chan struct{}
	result  chan pluginruntime.State
}

func (delivery *runtimeBoundDelivery) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}

func (delivery *runtimeBoundDelivery) DeliverEvent(ctx context.Context, _ chatevent.Event) (plugins.Delivery, error) {
	close(delivery.started)
	<-ctx.Done()
	// An IPC write cannot finish until the owning runtime stops.
	for delivery.manager.Snapshot().State != pluginruntime.StateStopped {
		time.Sleep(time.Millisecond)
	}
	delivery.result <- delivery.manager.Snapshot().State
	return plugins.Delivery{}, ctx.Err()
}

func TestAppCloseWaitsForDeliveriesAfterStoppingRuntimes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runtimes := pluginruntime.NewRegistry(logger, pluginruntime.Options{})
	manager := runtimes.GetOrCreate("fixture")
	manager.SetBackoffState(time.Now())
	delivery := &runtimeBoundDelivery{manager: manager, started: make(chan struct{}), result: make(chan pluginruntime.State, 1)}
	dispatcher := dispatch.New(logger, nil, nil, 4)
	dispatcher.Register("fixture", delivery, nil, nil, 1)
	application := &App{runtimes: runtimes, eventStack: EventState{Dispatcher: dispatcher}}
	dispatcher.DispatchToPlugin(t.Context(), "fixture", chatevent.Event{EventID: "fixture"})
	<-delivery.started
	done := make(chan error, 1)
	go func() { done <- application.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("app waited for delivery before stopping its runtime")
	}
	select {
	case state := <-delivery.result:
		if state != pluginruntime.StateStopped {
			t.Fatalf("runtime remained %s", state)
		}
	default:
		t.Fatal("app returned before delivery completed")
	}
}

func TestNewReleasesConfigLifecycleLockAfterBuildFailure(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config-as-directory")
	if err := os.MkdirAll(configPath, 0o755); err != nil {
		t.Fatalf("create invalid config directory: %v", err)
	}
	if _, err := New(Options{ConfigPath: configPath}); err == nil {
		t.Fatal("New should fail when the config path is a directory")
	}
	lockPath, err := runtimepaths.ResolveConfigLifecycleLockPath(configPath)
	if err != nil {
		t.Fatalf("resolve config lifecycle lock: %v", err)
	}
	lock, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("acquire lock after failed build: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatalf("close lock: %v", err)
	}
}

func TestAppCloseReleasesConfigLifecycleLock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "user.yaml.runtime.lock")
	lock, err := filelock.Acquire(path)
	if err != nil {
		t.Fatalf("acquire config lifecycle lock: %v", err)
	}
	application := &App{configLifecycleLock: lock}
	if _, err := filelock.Acquire(path); !errors.Is(err, filelock.ErrLocked) {
		t.Fatalf("overlapping lock error = %v, want ErrLocked", err)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("close app: %v", err)
	}
	reacquired, err := filelock.Acquire(path)
	if err != nil {
		t.Fatalf("reacquire released config lifecycle lock: %v", err)
	}
	if err := reacquired.Close(); err != nil {
		t.Fatalf("close reacquired lock: %v", err)
	}
}

func TestAppCloseReleasesThirdPartyQRCodeLoginSessions(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	provider := &appCloseQRProvider{session: thirdparty.QRLoginSession{
		Platform:  thirdparty.PlatformDouyin,
		Token:     "fixture-token",
		QRCodeURL: "https://example.test/qr",
		ExpiresAt: now.Add(3 * time.Minute),
		State:     thirdparty.QRLoginStatePendingScan,
	}}
	service := thirdparty.NewQRLoginService(
		map[string]thirdparty.QRLoginProvider{thirdparty.PlatformDouyin: provider},
		func() time.Time { return now },
	)
	if _, err := service.Create(context.Background(), thirdparty.PlatformDouyin); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	application := &App{services: Services{ThirdPartyQRLogin: service}}

	if err := application.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
	provider.mu.Lock()
	closeCalls := provider.closeCalls
	provider.mu.Unlock()
	if closeCalls != 1 {
		t.Fatalf("provider close calls = %d, want 1", closeCalls)
	}
}

type appCloseQRProvider struct {
	session thirdparty.QRLoginSession

	mu         sync.Mutex
	closeCalls int
}

func (p *appCloseQRProvider) Create(context.Context, time.Time) (thirdparty.QRLoginSession, error) {
	return p.session, nil
}

func (p *appCloseQRProvider) Poll(context.Context, thirdparty.QRLoginSession, time.Time) (thirdparty.QRLoginSession, error) {
	return p.session, nil
}

func (p *appCloseQRProvider) Close(thirdparty.QRLoginSession) {
	p.mu.Lock()
	p.closeCalls++
	p.mu.Unlock()
}

// ReadyForEvents reports whether this target can accept a plugin event.
func (delivery *runtimeBoundDelivery) ReadyForEvents() bool {
	return delivery.Snapshot().State == pluginruntime.StateRunning
}

func TestAppCloseRetainsTaskAndCloserErrorsAndReleasesRemainingResources(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "state.db")
	store, err := storage.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	lockPath := filepath.Join(root, "config.lock")
	lock, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Close() })
	renderErr := errors.New("runner close failed")
	runner := &appCloseRunner{err: renderErr}
	renderer, err := renderservice.NewService(renderservice.Options{
		RepoRoot: root, OutputRoot: filepath.Join(root, "render"), Store: store, Runner: runner,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = renderer.Close() })
	application := &App{
		platform: PlatformState{Storage: store}, renderStack: appRenderState{Renderer: renderer}, configLifecycleLock: lock,
	}
	supervisor := newRunSupervisor(t.Context())
	started := make(chan struct{})
	close(started)
	if !application.setRunSupervisor(supervisor, started) {
		t.Fatal("could not bind supervisor")
	}
	taskErr := errors.New("background task failed")
	supervisor.Go(func(context.Context) error { return taskErr })
	db := store.Read
	const closers = 8
	results := make(chan error, closers)
	for range closers {
		go func() { results <- application.Close() }()
	}
	for range closers {
		if err := <-results; !errors.Is(err, taskErr) || !errors.Is(err, renderErr) {
			t.Fatalf("Close() = %v, want task and render errors", err)
		}
	}
	if runner.closes.Load() != 1 {
		t.Fatalf("runner closed %d times, want 1", runner.closes.Load())
	}
	if err := db.PingContext(t.Context()); err == nil {
		t.Fatal("database remained usable after Close")
	}
	reopened, err := storage.Open(databasePath)
	if err != nil {
		t.Fatalf("database lock remained held: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	reacquired, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("config lifecycle lock remained held: %v", err)
	}
	if err := reacquired.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAppCloseWaitsForDatabaseWorkers(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	application := &App{platform: PlatformState{Storage: store}}
	supervisor := newRunSupervisor(t.Context())
	started := make(chan struct{})
	close(started)
	if !application.setRunSupervisor(supervisor, started) {
		t.Fatal("could not bind supervisor")
	}
	supervisor.Go(func(ctx context.Context) error {
		<-ctx.Done()
		// A worker may still need its owned storage to finish after cancellation.
		return store.Read.PingContext(context.Background())
	})
	if err := application.Close(); err != nil {
		t.Fatalf("database closed before background worker completed: %v", err)
	}
}

type appCloseRunner struct {
	err    error
	closes atomic.Int32
}

func (*appCloseRunner) Render(context.Context, renderservice.Document) ([]byte, error) {
	return nil, errors.New("unexpected render")
}

func (runner *appCloseRunner) Close() error {
	runner.closes.Add(1)
	return runner.err
}

func TestAppCloseWaitsForInboundHandlerBeforeClosingDatabase(t *testing.T) {
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	database := store.Read
	shell := onebot11.New("fixture", config.OneBotConfig{}, config.AdapterConfig{}, nil)
	application := &App{platform: PlatformState{Storage: store}, eventStack: EventState{
		Adapter: shell, OneBotShells: map[string]*onebot11.Shell{"fixture": shell},
	}}
	started := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		_ = application.Close()
	})
	shell.SetEventHandler(func(ctx context.Context, _ chatevent.NormalizedEvent) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-release
		result <- database.PingContext(context.Background())
	})
	shell.Start(t.Context())
	payload := []byte(`{"post_type":"message","message_type":"group","message_id":123,"group_id":456,"user_id":789,"self_id":101,"message":"hello","raw_message":"hello","time":1700000000}`)
	if err := shell.AcceptWebhookPayload(t.Context(), payload); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("inbound handler did not start")
	}
	done := make(chan error, 1)
	go func() { done <- application.Close() }()
	<-cancelled
	select {
	case err := <-done:
		t.Fatalf("Close returned before its inbound handler finished: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatalf("inbound handler lost its database before finishing: %v", err)
	}
}
