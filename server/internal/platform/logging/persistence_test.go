package logging

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestSpoolQueueFlushesRecordsAndQuarantinesBadLines(t *testing.T) {
	t.Parallel()

	queue := NewSpoolQueue(filepath.Join(t.TempDir(), "management-logs.spool.jsonl"))
	if err := queue.Append(Summary{
		BootID:    "boot_old",
		LogID:     "log_spool_0001",
		Timestamp: "2026-04-15T00:00:01Z",
		Level:     "info",
		Source:    "runtime",
		Message:   "first",
	}); err != nil {
		t.Fatalf("append first spool record: %v", err)
	}

	file, err := os.OpenFile(queue.Path(), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open spool file: %v", err)
	}
	if _, err := file.Write([]byte("{not-json}\n")); err != nil {
		t.Fatalf("append bad spool line: %v", err)
	}
	_ = file.Close()

	if err := queue.Append(Summary{
		LogID:     "log_spool_0002",
		Timestamp: "2026-04-15T00:00:02Z",
		Level:     "warn",
		Source:    "runtime",
		Message:   "second",
	}); err != nil {
		t.Fatalf("append second spool record: %v", err)
	}

	repository := &recordingRepository{}
	result, err := queue.Flush(context.Background(), repository)
	if err != nil {
		t.Fatalf("flush spool queue: %v", err)
	}
	if result.Flushed != 2 || result.Quarantined != 1 || result.Pending != 0 {
		t.Fatalf("unexpected flush result: %#v", result)
	}
	if len(repository.saved) != 2 {
		t.Fatalf("unexpected saved summaries: %#v", repository.saved)
	}
	if repository.saved[0].BootID != "boot_old" {
		t.Fatalf("expected flushed spool record to keep boot id: %#v", repository.saved[0])
	}
	if queue.HasEntries() {
		t.Fatalf("spool queue should be empty after flush")
	}

	quarantineRaw, err := os.ReadFile(queue.QuarantinePath())
	if err != nil {
		t.Fatalf("read quarantine file: %v", err)
	}
	if string(quarantineRaw) != "{not-json}\n" {
		t.Fatalf("unexpected quarantine content: %q", string(quarantineRaw))
	}
}

func TestSpoolQueueKeepsPendingRecordsWhenRepositoryFails(t *testing.T) {
	t.Parallel()

	queue := NewSpoolQueue(filepath.Join(t.TempDir(), "management-logs.spool.jsonl"))
	for _, summary := range []Summary{
		{LogID: "log_spool_fail_0001", Timestamp: "2026-04-15T00:00:01Z", Level: "info", Source: "runtime", Message: "first"},
		{LogID: "log_spool_fail_0002", Timestamp: "2026-04-15T00:00:02Z", Level: "info", Source: "runtime", Message: "second"},
	} {
		if err := queue.Append(summary); err != nil {
			t.Fatalf("append spool record: %v", err)
		}
	}

	repository := &recordingRepository{saveErr: errors.New("database unavailable")}
	result, err := queue.Flush(context.Background(), repository)
	if err == nil {
		t.Fatal("expected flush error")
	}
	if result.Pending != 2 {
		t.Fatalf("unexpected pending result: %#v", result)
	}
	if !queue.HasEntries() {
		t.Fatalf("spool queue should keep pending records")
	}
}

func TestSpoolAppendDoesNotWaitForReplayAndSurvivesRewrite(t *testing.T) {
	for _, fail := range []bool{false, true} {
		name := "success"
		if fail {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			queue := NewSpoolQueue(filepath.Join(t.TempDir(), "spool.jsonl"))
			original := Summary{LogID: "original", Timestamp: "2026-10-07T00:00:00Z", Level: "info"}
			if err := queue.Append(original); err != nil {
				t.Fatal(err)
			}
			repository := &blockedSpoolRepository{started: make(chan struct{}), release: make(chan struct{})}
			if fail {
				repository.saveErr = errors.New("database unavailable")
			}
			var release sync.Once
			defer release.Do(func() { close(repository.release) })
			done := make(chan struct{})
			var result SpoolFlushResult
			var flushErr error
			go func() {
				result, flushErr = queue.Flush(t.Context(), repository)
				close(done)
			}()
			select {
			case <-repository.started:
			case <-time.After(time.Second):
				t.Fatal("spool replay did not start")
			}
			appended := make(chan error, 1)
			go func() {
				appended <- queue.Append(Summary{LogID: "overflow", Timestamp: original.Timestamp, Level: "info"})
			}()
			select {
			case err := <-appended:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("spool append waited for database replay")
			}
			release.Do(func() { close(repository.release) })
			<-done
			wantIDs := []string{"overflow"}
			wantFlushed := 1
			if fail {
				wantIDs = []string{"original", "overflow"}
				wantFlushed = 0
			}
			if (flushErr != nil) != fail || result.Flushed != wantFlushed || result.Pending != len(wantIDs) {
				t.Fatalf("replay result: %+v, %v", result, flushErr)
			}
			remaining := &recordingRepository{}
			if _, err := queue.Flush(t.Context(), remaining); err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, summary := range remaining.saved {
				ids = append(ids, summary.LogID)
			}
			if !slices.Equal(ids, wantIDs) || queue.HasEntries() {
				t.Fatalf("pending records changed during replay: %v, want %v", ids, wantIDs)
			}
		})
	}
}

type blockedSpoolRepository struct {
	recordingRepository
	started chan struct{}
	release chan struct{}
}

func (r *blockedSpoolRepository) SaveSummary(ctx context.Context, summary Summary) error {
	close(r.started)
	select {
	case <-r.release:
		return r.recordingRepository.SaveSummary(ctx, summary)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestStreamAppendsAfterSpoolingWhenDatabaseFails(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	stream := NewStream(8)
	t.Cleanup(stream.Close)

	queue := NewSpoolQueue(filepath.Join(tempDir, "management-logs.spool.jsonl"))
	stream.ConfigureSpool(queue, io.Discard)
	stream.SetRepository(&recordingRepository{saveErr: errors.New("database unavailable")}, 0)

	summaries, unsubscribe := stream.Subscribe(1)
	defer unsubscribe()

	stream.Append(Summary{
		LogID:     "log_stream_spool_0001",
		Timestamp: "2026-04-15T00:00:01Z",
		Level:     "info",
		Source:    "runtime",
		Message:   "spooled",
	})

	select {
	case summary := <-summaries:
		if summary.LogID != "log_stream_spool_0001" {
			t.Fatalf("unexpected streamed summary: %#v", summary)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for streamed summary")
	}

	if !queue.HasEntries() {
		t.Fatalf("expected summary to be written into spool queue")
	}
}

func TestStreamDropsLogWhenDatabaseAndSpoolBothFail(t *testing.T) {
	t.Parallel()

	blockedPath := filepath.Join(t.TempDir(), "blocked-parent")
	if err := os.WriteFile(blockedPath, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("create blocked path: %v", err)
	}

	stream := NewStream(8)
	t.Cleanup(stream.Close)

	stream.ConfigureSpool(NewSpoolQueue(filepath.Join(blockedPath, "management-logs.spool.jsonl")), io.Discard)
	stream.SetRepository(&recordingRepository{saveErr: errors.New("database unavailable")}, 0)

	summaries, unsubscribe := stream.Subscribe(1)
	defer unsubscribe()

	stream.Append(Summary{
		LogID:     "log_stream_drop_0001",
		Timestamp: "2026-04-15T00:00:01Z",
		Level:     "info",
		Source:    "runtime",
		Message:   "drop me",
	})

	if err := stream.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}

	select {
	case summary := <-summaries:
		t.Fatalf("unexpected streamed summary after full persistence failure: %#v", summary)
	default:
	}

	if len(stream.Snapshot()) != 0 {
		t.Fatalf("stream snapshot should stay empty after full persistence failure")
	}
}

func TestStreamFlushesQueuedRecordsOnceRepositoryRecovers(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	stream := NewStream(8)
	t.Cleanup(stream.Close)

	queue := NewSpoolQueue(filepath.Join(tempDir, "management-logs.spool.jsonl"))
	if err := queue.Append(Summary{
		LogID:     "log_stream_flush_0001",
		Timestamp: "2026-04-15T00:00:01Z",
		Level:     "info",
		Source:    "runtime",
		Message:   "flush me",
	}); err != nil {
		t.Fatalf("append initial spool record: %v", err)
	}

	repository := &recordingRepository{}
	stream.ConfigureSpool(queue, io.Discard)
	stream.SetRepository(repository, 0)

	if err := stream.FlushSpool(context.Background()); err != nil {
		t.Fatalf("flush spool via stream: %v", err)
	}
	if len(repository.saved) != 1 {
		t.Fatalf("unexpected saved summaries after flush: %#v", repository.saved)
	}
	if queue.HasEntries() {
		t.Fatalf("spool queue should be empty after stream flush")
	}
}

type recordingRepository struct {
	saved   []Summary
	saveErr error
}

func (r *recordingRepository) SaveSummary(_ context.Context, summary Summary) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = append(r.saved, NormalizeSummary(summary))
	return nil
}

func (r *recordingRepository) SaveSummaries(ctx context.Context, summaries []Summary) error {
	for _, summary := range summaries {
		if err := r.SaveSummary(ctx, summary); err != nil {
			return err
		}
	}
	return nil
}

func (*recordingRepository) ListSummaries(context.Context, Query) ([]Summary, error) {
	return nil, nil
}

func (*recordingRepository) ListPage(context.Context, PageQuery) (PageResult, error) {
	return PageResult{}, nil
}

func (*recordingRepository) GetSummary(context.Context, string) (Summary, error) {
	return Summary{}, ErrLogNotFound
}

func (*recordingRepository) PruneOlderThan(context.Context, time.Time) error {
	return nil
}
