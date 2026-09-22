package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestLogOutputAndStorageUseUTCWithoutChangingSourceTime(t *testing.T) {
	for _, bootstrap := range []bool{false, true} {
		for _, offset := range []int{8 * 3600, -7 * 3600} {
			var output bytes.Buffer
			var level slog.LevelVar
			logger := newLoggerWithLevelVar(&output, &level)
			if bootstrap {
				logger = newLoggerWithWriter(slog.LevelInfo, &output)
			}
			instant := time.Date(2026, 1, 15, 20, 30, 0, 123456789, time.UTC).In(time.FixedZone("source", offset))
			record := slog.NewRecord(instant, slog.LevelInfo, "timezone check", 0)
			record.AddAttrs(slog.Int64("event_timestamp", 1768508990))
			if err := logger.Handler().Handle(context.Background(), record); err != nil {
				t.Fatal(err)
			}
			var body map[string]any
			if err := json.Unmarshal(output.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body["ts"] != "2026-01-15T20:30:00.123456789Z" || body["event_timestamp"] != float64(1768508990) {
				t.Fatalf("log timestamps = %#v", body)
			}
			summary, ok := summaryFromJSONLine(output.Bytes())
			if !ok || summary.Timestamp != body["ts"] || summary.Details["event_timestamp"] != body["event_timestamp"] {
				t.Fatalf("stored timestamps = %+v", summary)
			}
		}
	}
}

func TestNormalizeHistoricalLogTimestampsToFixedUTC(t *testing.T) {
	for input, want := range map[string]string{
		"2026-01-16T04:30:00+08:00":           "2026-01-15T20:30:00.000000000Z",
		"2026-01-15T20:30:00.1Z":              "2026-01-15T20:30:00.100000000Z",
		"2026-11-01T01:30:00.123456789-04:00": "2026-11-01T05:30:00.123456789Z",
		"2026-11-01T01:30:00.123456789-05:00": "2026-11-01T06:30:00.123456789Z",
	} {
		if got := NormalizeSummary(Summary{Timestamp: input}).Timestamp; got != want {
			t.Fatalf("normalize %q = %q, want %q", input, got, want)
		}
	}
}
