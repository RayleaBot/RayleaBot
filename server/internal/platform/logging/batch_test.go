package logging

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStreamCloseDrainsInOrderAndLaterLogsOnlyReachStdout(t *testing.T) {
	stream := NewStream(600)
	repository := &recordingRepository{}
	stream.SetRepository(repository, 7)
	t.Cleanup(stream.Close)
	for index := range 600 {
		stream.Append(Summary{LogID: fmt.Sprint(index), Timestamp: "2026-10-07T00:00:00Z", Level: "info", Message: "fixture"})
	}
	stream.Close()
	if len(repository.saved) != 600 || len(stream.Snapshot()) != 600 {
		t.Fatalf("shutdown lost queued logs: stored=%d history=%d", len(repository.saved), len(stream.Snapshot()))
	}
	for index, item := range repository.saved {
		if item.LogID != fmt.Sprint(index) || stream.Snapshot()[index].LogID != item.LogID {
			t.Fatalf("queued log order changed at %d", index)
		}
	}
	var output bytes.Buffer
	writer := NewSummaryWriter(&output, stream, nil)
	if _, err := writer.Write([]byte("{\"ts\":\"2026-10-07T00:00:00Z\",\"level\":\"info\",\"msg\":\"late\"}\n")); err != nil {
		t.Fatal(err)
	}
	if err := stream.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if output.Len() == 0 || len(repository.saved) != 600 || len(stream.Snapshot()) != 600 {
		t.Fatal("log after close did not remain stdout-only")
	}
}

func TestStreamQueueOverflowSpoolsWithoutWaitingForDatabase(t *testing.T) {
	stream := NewStream(writeQueueCapacity + writeBatchSize + 1)
	repository := &gatedBatchRepository{started: make(chan struct{}), release: make(chan struct{})}
	queue := NewSpoolQueue(filepath.Join(t.TempDir(), "spool.jsonl"))
	stream.ConfigureSpool(queue, io.Discard)
	stream.SetRepository(repository, 0)
	var release sync.Once
	t.Cleanup(func() { release.Do(func() { close(repository.release) }); stream.Close() })
	for index := range writeBatchSize {
		stream.Append(Summary{LogID: fmt.Sprint(index), Timestamp: "2026-10-07T00:00:00Z", Level: "info", Message: "fixture"})
	}
	select {
	case <-repository.started:
	case <-time.After(time.Second):
		t.Fatal("database write did not start")
	}
	for index := range writeQueueCapacity {
		stream.Append(Summary{LogID: fmt.Sprint(index + writeBatchSize), Timestamp: "2026-10-07T00:00:00Z", Level: "info", Message: "fixture"})
	}
	done := make(chan struct{})
	go func() {
		stream.Append(Summary{LogID: "overflow", Timestamp: "2026-10-07T00:00:00Z", Level: "info", Message: "fixture"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("overflow waited for the database")
	}
	history := stream.Snapshot()
	if len(history) != 1 || history[0].LogID != "overflow" || !queue.HasEntries() {
		t.Fatalf("overflow was not persisted and published: %+v", history)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := stream.Flush(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("blocked flush ignored cancellation: %v", err)
	}
	release.Do(func() { close(repository.release) })
	if err := stream.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(stream.Snapshot()) != writeQueueCapacity+writeBatchSize+1 {
		t.Fatal("draining the full queue lost logs")
	}
}

type gatedBatchRepository struct {
	recordingRepository
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *gatedBatchRepository) SaveSummaries(ctx context.Context, summaries []Summary) error {
	r.once.Do(func() { close(r.started) })
	select {
	case <-r.release:
		return r.recordingRepository.SaveSummaries(ctx, summaries)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (*gatedBatchRepository) SaveSummary(context.Context, Summary) error {
	return errors.New("spool replay unavailable")
}

func TestFailedBatchSpoolsEveryRecordBeforePublication(t *testing.T) {
	stream := NewStream(8)
	queue := NewSpoolQueue(filepath.Join(t.TempDir(), "spool.jsonl"))
	stream.ConfigureSpool(queue, io.Discard)
	stream.SetRepository(&recordingRepository{saveErr: errors.New("database unavailable")}, 7)
	t.Cleanup(stream.Close)
	for index := range 3 {
		stream.Append(Summary{LogID: fmt.Sprint(index), Timestamp: "2026-10-07T00:00:00Z", Level: "info", Message: "fixture"})
	}
	stream.Close()
	replayed := &recordingRepository{}
	result, err := queue.Flush(t.Context(), replayed)
	if err != nil || result.Flushed != 3 || len(stream.Snapshot()) != 3 {
		t.Fatalf("failed batch was not fully spooled: %+v, %v", result, err)
	}
	for index, item := range replayed.saved {
		if item.LogID != fmt.Sprint(index) || stream.Snapshot()[index].LogID != item.LogID {
			t.Fatalf("failed batch order changed at %d", index)
		}
	}
}

func TestCloseBudgetCancelsDatabaseWaitAndSpoolsRemainingLogs(t *testing.T) {
	stream := NewStream(8)
	queue := NewSpoolQueue(filepath.Join(t.TempDir(), "spool.jsonl"))
	repository := &gatedBatchRepository{started: make(chan struct{}), release: make(chan struct{})}
	stream.ConfigureSpool(queue, io.Discard)
	stream.SetRepository(repository, 0)
	t.Cleanup(stream.Close)
	stream.Append(Summary{LogID: "pending", Timestamp: "2026-10-07T00:00:00Z", Level: "info", Message: "fixture"})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	closed := make(chan struct{})
	go func() { stream.CloseContext(ctx); close(closed) }()
	select {
	case <-repository.started:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not try to save pending logs")
	}
	cancel()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("shutdown kept waiting for the database after its budget expired")
	}
	replayed := &recordingRepository{}
	if result, err := queue.Flush(t.Context(), replayed); err != nil || result.Flushed != 1 || len(stream.Snapshot()) != 1 {
		t.Fatalf("shutdown lost its canceled batch: %+v, %v", result, err)
	}
}
