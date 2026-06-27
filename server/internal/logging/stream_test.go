package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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

func TestSummaryWriterKeepsLogTimestampSeparateFromOneBotTimeDetail(t *testing.T) {
	t.Parallel()

	stream := NewStream(8)
	var output bytes.Buffer
	logger := newLoggerWithWriter(slog.LevelInfo, NewSummaryWriter(&output, stream, nil))

	logger.Info(
		"10001: [测试群(20001)]测试用户A(3001): hello bridge",
		"component", "bridge",
		"time", int64(1710000900),
		"message_id", "40002",
	)

	var body map[string]any
	if err := json.Unmarshal(output.Bytes(), &body); err != nil {
		t.Fatalf("decode structured log line: %v", err)
	}
	if _, ok := body["ts"].(string); !ok {
		t.Fatalf("expected built-in log timestamp string, got %#v", body["ts"])
	}
	if body["time"] != float64(1710000900) {
		t.Fatalf("expected preserved onebot time detail, got %#v", body["time"])
	}
	if got := body["request_id"]; got != defaultRequestID {
		t.Fatalf("unexpected default request_id: got %#v want %#v", got, defaultRequestID)
	}

	summaries := stream.Snapshot()
	if len(summaries) != 1 {
		t.Fatalf("unexpected summary count: got %d want 1", len(summaries))
	}
	if summaries[0].Timestamp == "1710000900" || summaries[0].Timestamp == "" {
		t.Fatalf("expected RFC3339 log timestamp, got %#v", summaries[0].Timestamp)
	}
	if got := summaries[0].Details["time"]; got != float64(1710000900) {
		t.Fatalf("expected summary details to keep onebot time, got %#v", got)
	}
	if got := summaries[0].RequestID; got != defaultRequestID {
		t.Fatalf("unexpected summary request_id: got %#v want %#v", got, defaultRequestID)
	}
}

func TestLoggerAddsDefaultRequestID(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := newLoggerWithWriter(slog.LevelInfo, &output)

	logger.Info("运行时已启动：测试日志", "component", "runtime")

	var body map[string]any
	if err := json.Unmarshal(output.Bytes(), &body); err != nil {
		t.Fatalf("decode structured log line: %v", err)
	}
	if got := body["request_id"]; got != defaultRequestID {
		t.Fatalf("unexpected default request_id: got %#v want %#v", got, defaultRequestID)
	}
}

func TestLoggerKeepsExplicitRequestID(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := newLoggerWithWriter(slog.LevelInfo, &output)

	logger.Info("请求已完成：req_fixture_001", "request_id", "req_fixture_001")

	line := output.String()
	if count := strings.Count(line, `"request_id"`); count != 1 {
		t.Fatalf("unexpected request_id key count: got %d in %s", count, line)
	}

	var body map[string]any
	if err := json.Unmarshal(output.Bytes(), &body); err != nil {
		t.Fatalf("decode structured log line: %v", err)
	}
	if got := body["request_id"]; got != "req_fixture_001" {
		t.Fatalf("unexpected request_id: got %#v", got)
	}
}

func TestLoggerKeepsScopedRequestID(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := newLoggerWithWriter(slog.LevelInfo, &output).With("request_id", "req_scoped_001")

	logger.Info("带请求上下文的测试日志")

	line := output.String()
	if count := strings.Count(line, `"request_id"`); count != 1 {
		t.Fatalf("unexpected request_id key count: got %d in %s", count, line)
	}

	var body map[string]any
	if err := json.Unmarshal(output.Bytes(), &body); err != nil {
		t.Fatalf("decode structured log line: %v", err)
	}
	if got := body["request_id"]; got != "req_scoped_001" {
		t.Fatalf("unexpected scoped request_id: got %#v", got)
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
			Message:   "运行时桥接已投递群消息",
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

func TestStreamAppendInjectsCurrentBootID(t *testing.T) {
	t.Parallel()

	stream := NewStream(8)
	stream.SetBootID("boot_current")

	stream.Append(Summary{
		LogID:     "log_boot_stream_0001",
		Timestamp: "2026-04-09T20:42:01Z",
		Level:     "info",
		Source:    "runtime",
		Message:   "带启动批次的日志摘要",
	})

	summaries := stream.Snapshot()
	if len(summaries) != 1 {
		t.Fatalf("unexpected summary count: got %d want 1", len(summaries))
	}
	if summaries[0].BootID != "boot_current" {
		t.Fatalf("unexpected boot id: %#v", summaries[0])
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

func (*blockingRepository) ListPage(context.Context, PageQuery) (PageResult, error) {
	return PageResult{}, nil
}

func (*blockingRepository) GetSummary(context.Context, string) (Summary, error) {
	return Summary{}, ErrLogNotFound
}

func (*blockingRepository) PruneOlderThan(context.Context, time.Time) error {
	return nil
}
