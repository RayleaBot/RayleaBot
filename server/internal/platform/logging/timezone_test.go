package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestLogOutputUsesConfiguredTimezoneAndStorageSummaryUsesUTC(t *testing.T) {
	var output bytes.Buffer
	var level slog.LevelVar
	logger := newLoggerWithLevelVar(&output, &level, time.FixedZone("UTC+8", 8*60*60))
	instant := time.Date(2026, 1, 15, 20, 30, 0, 0, time.UTC)
	if err := logger.Handler().Handle(context.Background(), slog.NewRecord(instant, slog.LevelInfo, "timezone check", 0)); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(output.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ts"] != "2026-01-16T04:30:00+08:00" {
		t.Fatalf("display timestamp = %v", body["ts"])
	}
	summary, ok := summaryFromJSONLine(output.Bytes())
	if !ok || summary.Timestamp != "2026-01-15T20:30:00Z" {
		t.Fatalf("stored timestamp = %+v", summary)
	}
}
