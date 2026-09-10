package storage

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRunSnapshotLoopWaitsForSnapshotBeforeReturning(t *testing.T) {
	root := t.TempDir()
	store, err := Open(filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	writer := &snapshotBlockingWriter{started: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(writer.release) }) })
	logger := slog.New(slog.NewTextHandler(writer, nil))
	done := make(chan struct{})
	go func() {
		RunSnapshotLoop(ctx, store, logger, root)
		close(done)
	}()
	select {
	case <-writer.started:
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot did not complete")
	}
	cancel()
	select {
	case <-done:
		t.Fatal("snapshot loop returned while snapshot work was still running")
	default:
	}
	release.Do(func() { close(writer.release) })
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("snapshot loop did not return after cancellation")
	}
	files, err := filepath.Glob(filepath.Join(SnapshotDirForDatabase(store.Path), "*.db"))
	if err != nil || len(files) != 1 {
		t.Fatalf("snapshot files = %v, error = %v; want one completed snapshot", files, err)
	}
}

type snapshotBlockingWriter struct {
	started chan struct{}
	release chan struct{}
}

func (writer *snapshotBlockingWriter) Write(data []byte) (int, error) {
	close(writer.started)
	<-writer.release
	return io.Discard.Write(data)
}
