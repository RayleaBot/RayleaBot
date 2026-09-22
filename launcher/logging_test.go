package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestLauncherRuntimeLoggerUsesFixedUTC(t *testing.T) {
	var output bytes.Buffer
	logger := newLauncherLogger(&output)
	instant := time.Date(2026, 9, 23, 0, 0, 0, 123456789, time.FixedZone("source", 8*3600))
	if err := logger.Handler().Handle(t.Context(), slog.NewRecord(instant, slog.LevelInfo, "runtime ready", 0)); err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["ts"] != "2026-09-22T16:00:00.123456789Z" || record["component"] != "launcher" {
		t.Fatalf("launcher timestamp = %#v", record)
	}
}
