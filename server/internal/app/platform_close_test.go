package app

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	loggingsqlite "github.com/RayleaBot/RayleaBot/server/internal/platform/logging/sqlite"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
)

func TestClosePersistenceDrainsLogsBeforeClosingStorage(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "state.db")
	store, err := storage.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	base, err := loggingsqlite.NewRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	repository := &platformBlockingLogRepository{Repository: base, started: make(chan struct{}), release: make(chan struct{})}
	logs := logging.NewStream(10)
	logs.SetRepository(repository, 7)
	application := &App{platform: PlatformState{Storage: store, Logs: logs}}
	var release sync.Once
	t.Cleanup(func() {
		release.Do(func() { close(repository.release) })
		_ = application.closePersistence(t.Context())
	})
	logs.Append(logging.Summary{LogID: "last-log", Timestamp: logging.FormatTimestamp(time.Now()), Level: "info", Message: "fixture"})
	closed := make(chan error, 1)
	go func() { closed <- application.closePersistence(t.Context()) }()
	select {
	case <-repository.started:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not drain the queue")
	}
	if reopened, err := storage.Open(databasePath); err == nil {
		_ = reopened.Close()
		t.Fatal("database was released before the queued log finished")
	}
	release.Do(func() { close(repository.release) })
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	reopened, err := storage.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()
	persisted, err := loggingsqlite.NewRepository(reopened)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := persisted.GetSummary(t.Context(), "last-log"); err != nil {
		t.Fatalf("queued log lost at shutdown: %v", err)
	}
}

type platformBlockingLogRepository struct {
	logging.Repository
	started chan struct{}
	release chan struct{}
}

func (r *platformBlockingLogRepository) SaveSummaries(ctx context.Context, summaries []logging.Summary) error {
	close(r.started)
	select {
	case <-r.release:
		return r.Repository.SaveSummaries(ctx, summaries)
	case <-ctx.Done():
		return ctx.Err()
	}
}
