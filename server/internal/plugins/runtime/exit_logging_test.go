package runtime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestInterruptedEventExitIsReportedOnce(t *testing.T) {
	for _, readFirst := range []bool{true, false} {
		for _, crashed := range []bool{false, true} {
			name := "clean"
			if crashed {
				name = "crashed"
			}
			if readFirst {
				name += "/stdout-first"
			} else {
				name += "/process-first"
			}
			t.Run(name, func(t *testing.T) {
				var logs bytes.Buffer
				manager := NewManager(slog.New(slog.NewJSONHandler(&logs, nil)), Options{})
				stdinReader, stdinWriter := io.Pipe()
				stdoutReader, stdoutWriter := io.Pipe()
				t.Cleanup(func() {
					_ = stdinReader.Close()
					_ = stdinWriter.Close()
					_ = stdoutReader.Close()
					_ = stdoutWriter.Close()
				})
				handle := NewHandle(nil, stdinWriter, bufio.NewReader(stdoutReader), ProcessSpec{PluginID: "fixture", EventTimeout: time.Second})
				manager.proc = handle
				manager.snap.State = StateRunning
				finished := make(chan error, 1)
				go func() { _, err := manager.DeliverEvent(context.Background(), testRuntimeEvent()); finished <- err }()
				var event map[string]any
				if err := json.NewDecoder(stdinReader).Decode(&event); err != nil {
					t.Fatal(err)
				}
				readDone := make(chan struct{})
				go func() { manager.readRuntimeFrames(handle); close(readDone) }()
				var exitErr error
				if crashed {
					exitErr = errors.New("fixture process failure")
				}
				if readFirst {
					_ = stdoutWriter.Close()
					<-readDone
					handle.SetExit(exitErr)
					manager.watchRunningProcess(handle)
				} else {
					handle.SetExit(exitErr)
					watchDone := make(chan struct{})
					go func() { manager.watchRunningProcess(handle); close(watchDone) }()
					_ = stdoutWriter.Close()
					<-readDone
					<-watchDone
				}
				err := <-finished
				assertRuntimeErrorCode(t, err, codePluginInternalError)
				var failure *plugins.Error
				if !errors.As(err, &failure) || !failure.FailureReported() {
					t.Fatalf("unowned delivery error: %v", err)
				}
				warnings := 0
				decoder := json.NewDecoder(&logs)
				for decoder.More() {
					var record map[string]any
					if err := decoder.Decode(&record); err != nil {
						t.Fatal(err)
					}
					if record["level"] == "WARN" {
						warnings++
						if record["error_code"] != codePluginInternalError || record["plugin_id"] != "fixture" {
							t.Fatalf("missing failure identity: %#v", record)
						}
					}
				}
				if warnings != 1 {
					t.Fatalf("owning failure records = %d, want 1", warnings)
				}
			})
		}
	}
}
