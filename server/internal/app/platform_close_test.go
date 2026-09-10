package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestBuildPlatformWaitsForLogFlushBeforeClosingStorage(t *testing.T) {
	root := t.TempDir()
	databasePath := filepath.Join(root, "state.db")
	logs := logging.NewStream(10)
	registry := tasks.NewRegistry()
	executor := tasks.NewExecutor(registry, time.Second)
	expected := errors.New("retention failed")
	repository := &platformFailingLogRepository{
		logs: logs, err: expected, flushing: make(chan struct{}), release: make(chan struct{}), failed: make(chan struct{}),
	}
	var release sync.Once
	t.Cleanup(func() {
		release.Do(func() { close(repository.release) })
		_ = executor.Close()
		_ = registry.Close()
		logs.Close()
	})
	done := make(chan error, 1)
	go func() {
		_, err := buildPlatform(platformDeps{
			Context: t.Context(), ConfigPath: filepath.Join(root, "user.yaml"),
			Config: config.Config{
				Database: config.DatabaseConfig{Path: databasePath},
				Admin:    config.AdminConfig{SessionTTLDays: 7, SessionAbsoluteTTLDays: 30, MaxSessions: 3},
				Log:      config.LogConfig{RetentionDays: 7},
			},
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Tasks: registry,
			TaskExecutor: executor, Logs: logs, LogRepository: repository,
		})
		done <- err
	}()
	select {
	case <-repository.failed:
	case err := <-done:
		t.Fatalf("platform failed before exercising log flush: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("log spool did not begin flushing")
	}
	select {
	case err := <-done:
		t.Fatalf("construction returned before its log worker exited: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	if reopened, err := storage.Open(databasePath); err == nil {
		_ = reopened.Close()
		t.Fatal("database was released while its log worker was still flushing")
	}
	release.Do(func() { close(repository.release) })
	select {
	case err := <-done:
		if !errors.Is(err, expected) {
			t.Fatalf("buildPlatform() = %v, want retention failure", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("construction cleanup did not finish after log worker exited")
	}
	store, err := storage.Open(databasePath)
	if err != nil {
		t.Fatalf("database remained locked: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}

type platformFailingLogRepository struct {
	logging.Repository
	logs     *logging.Stream
	err      error
	flushing chan struct{}
	release  chan struct{}
	failed   chan struct{}
	saves    atomic.Int32
	prune    sync.Once
}

func (repository *platformFailingLogRepository) SaveSummary(ctx context.Context, _ logging.Summary) error {
	if repository.saves.Add(1) == 1 {
		return errors.New("write failed, queue for retry")
	}
	close(repository.flushing)
	select {
	case <-repository.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (repository *platformFailingLogRepository) PruneOlderThan(context.Context, time.Time) error {
	repository.prune.Do(func() {
		repository.logs.Append(logging.Summary{LogID: "fixture", Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Level: "INFO", Message: "queued"})
		<-repository.flushing
		close(repository.failed)
	})
	return repository.err
}
