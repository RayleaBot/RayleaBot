package logging

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSpoolQueueFlushesRecordsAndQuarantinesBadLines(t *testing.T) {
	t.Parallel()

	queue := NewSpoolQueue(filepath.Join(t.TempDir(), "management-logs.spool.jsonl"))
	if err := queue.Append(Summary{
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
	file.Close()

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

func TestStreamAppendsAfterSpoolingWhenDatabaseFails(t *testing.T) {
	t.Parallel()

	stream := NewStream(8)
	t.Cleanup(stream.Close)

	queue := NewSpoolQueue(filepath.Join(t.TempDir(), "management-logs.spool.jsonl"))
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

	select {
	case summary := <-summaries:
		t.Fatalf("unexpected streamed summary after full persistence failure: %#v", summary)
	case <-time.After(150 * time.Millisecond):
	}

	if len(stream.Snapshot()) != 0 {
		t.Fatalf("stream snapshot should stay empty after full persistence failure")
	}
}

func TestStreamFlushesQueuedRecordsOnceRepositoryRecovers(t *testing.T) {
	t.Parallel()

	stream := NewStream(8)
	t.Cleanup(stream.Close)

	queue := NewSpoolQueue(filepath.Join(t.TempDir(), "management-logs.spool.jsonl"))
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
