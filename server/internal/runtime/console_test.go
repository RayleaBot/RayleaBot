package runtime

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"rayleabot/server/internal/console"
)

func TestManagerCapturesRedactedConsoleFrames(t *testing.T) {
	t.Parallel()

	consoleStream := console.NewStream(1000, 2*1024*1024)
	manager := newManager(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		managerDeps{
			now: func() time.Time {
				return time.Unix(1_700_000_000, 0).UTC()
			},
			requestID: func() string {
				return "req_console_test"
			},
		},
		Options{
			Console: consoleStream,
			RedactText: func(text string) string {
				return strings.ReplaceAll(text, "fixture-only-secret", "[REDACTED]")
			},
			StderrRateLimitBytesPerSec: 262144,
		},
	)

	spec := helperSpec(t, "stderr-secret", "")
	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime with redacted stderr: %v", err)
	}
	defer func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Fatalf("stop runtime: %v", err)
		}
	}()

	entries := waitForConsoleEntries(t, consoleStream, "helper-plugin", 1)
	joined := joinConsoleText(entries)
	if strings.Contains(joined, "fixture-only-secret") {
		t.Fatalf("console output leaked secret: %s", joined)
	}
	if !strings.Contains(joined, "[REDACTED]") {
		t.Fatalf("expected redacted console output, got %s", joined)
	}
}

func TestManagerRateLimitsConsoleFrames(t *testing.T) {
	t.Parallel()

	consoleStream := console.NewStream(1000, 2*1024*1024)
	manager := newManager(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		managerDeps{
			now: func() time.Time {
				return time.Unix(1_700_000_000, 0).UTC()
			},
			requestID: func() string {
				return "req_console_limit"
			},
		},
		Options{
			Console:                    consoleStream,
			StderrRateLimitBytesPerSec: 16,
		},
	)

	spec := helperSpec(t, "stderr-noise", "")
	if err := manager.Start(context.Background(), spec, testInitPayload()); err != nil {
		t.Fatalf("start runtime with rate-limited stderr: %v", err)
	}
	defer func() {
		if err := manager.Stop(context.Background()); err != nil {
			t.Fatalf("stop runtime: %v", err)
		}
	}()

	entries := waitForConsoleEntries(t, consoleStream, "helper-plugin", 2)
	var stderrBytes int
	foundSystem := false
	for _, entry := range entries {
		switch entry.Stream {
		case "stderr":
			stderrBytes += len([]byte(entry.Text))
		case "system":
			if entry.Text == stderrTruncatedSystemMessage {
				foundSystem = true
			}
		}
	}

	if !foundSystem {
		t.Fatalf("expected console system truncation message, got %#v", entries)
	}
	if stderrBytes > 16 {
		t.Fatalf("stderr bytes exceeded configured limit: got %d want <= 16", stderrBytes)
	}
}

func waitForConsoleEntries(t *testing.T, stream *console.Stream, pluginID string, minimum int) []console.Entry {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		entries := stream.Snapshot(pluginID)
		if len(entries) >= minimum {
			return entries
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for console entries for %s", pluginID)
	return nil
}

func joinConsoleText(entries []console.Entry) string {
	var builder strings.Builder
	for _, entry := range entries {
		builder.WriteString(entry.Text)
	}
	return builder.String()
}
