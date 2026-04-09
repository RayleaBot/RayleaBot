package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestSummaryWriterRedactsStructuredLogValues(t *testing.T) {
	t.Parallel()

	stream := NewStream(8)
	var output bytes.Buffer
	writer := NewSummaryWriter(&output, stream, func(text string) string {
		return strings.ReplaceAll(text, "fixture-only-secret", "[REDACTED]")
	})

	line := `{"ts":"2026-03-20T10:00:00Z","level":"ERROR","component":"runtime","msg":"stderr leaked fixture-only-secret","token":"fixture-only-secret"}` + "\n"
	if _, err := writer.Write([]byte(line)); err != nil {
		t.Fatalf("write structured log line: %v", err)
	}

	raw := output.String()
	if strings.Contains(raw, "fixture-only-secret") {
		t.Fatalf("raw structured log output leaked secret: %s", raw)
	}
	if !strings.Contains(raw, "[REDACTED]") {
		t.Fatalf("expected redacted output, got %s", raw)
	}

	summaries := stream.Snapshot()
	if len(summaries) != 1 {
		t.Fatalf("unexpected summary count: got %d want %d", len(summaries), 1)
	}
	if strings.Contains(summaries[0].Message, "fixture-only-secret") {
		t.Fatalf("summary message leaked secret: %#v", summaries[0])
	}
}

func TestStreamAppendsAfterSavingRepositoryDetail(t *testing.T) {
	t.Parallel()

	stream := NewStream(8)
	repository := &blockingRepository{
		saveStarted: make(chan struct{}, 1),
		releaseSave: make(chan struct{}),
	}
	stream.SetRepository(repository, 0)

	summaries, unsubscribe := stream.Subscribe(1)
	defer unsubscribe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		stream.Append(Summary{
			LogID:     "log_live_0001",
			Timestamp: "2026-04-09T20:42:01Z",
			Level:     "info",
			Source:    "bridge",
			Message:   "runtime bridge delivered group message",
		})
	}()

	select {
	case <-repository.saveStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for repository save")
	}

	select {
	case <-summaries:
		t.Fatal("live summary should not be delivered before repository save finishes")
	case <-time.After(50 * time.Millisecond):
	}

	close(repository.releaseSave)

	select {
	case summary := <-summaries:
		if summary.LogID != "log_live_0001" {
			t.Fatalf("unexpected log summary after save: %#v", summary)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for live summary after repository save")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for append completion")
	}
}

type blockingRepository struct {
	saveStarted chan struct{}
	releaseSave chan struct{}
}

func (r *blockingRepository) SaveSummary(context.Context, Summary) error {
	select {
	case r.saveStarted <- struct{}{}:
	default:
	}
	<-r.releaseSave
	return nil
}

func (*blockingRepository) ListSummaries(context.Context, Query) ([]Summary, error) {
	return nil, nil
}

func (*blockingRepository) GetSummary(context.Context, string) (Summary, error) {
	return Summary{}, ErrLogNotFound
}

func (*blockingRepository) PruneOlderThan(context.Context, time.Time) error {
	return nil
}
