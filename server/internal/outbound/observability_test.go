package outbound

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

func newObservabilityTestLogger() (*slog.Logger, *logging.Stream) {
	stream := logging.NewStream(16)
	writer := logging.NewSummaryWriter(io.Discard, stream, nil)
	logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "ts"
			case slog.MessageKey:
				attr.Key = "msg"
			}
			return attr
		},
	}))
	return logger, stream
}

func waitForOutboundSummary(t *testing.T, stream *logging.Stream) logging.Summary {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		items := stream.Snapshot()
		if len(items) > 0 {
			return items[len(items)-1]
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for outbound log summary")
	return logging.Summary{}
}

func TestLogSendOutcomeUsesPlatformSummaryWithoutPluginContext(t *testing.T) {
	t.Parallel()

	logger, stream := newObservabilityTestLogger()

	LogSendOutcome(logger, SendLogContext{}, SendAttempt{
		ActionKind: "message.send",
		TargetType: "group",
		TargetID:   "200",
		Segments: []adapter.OutboundMessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "cooldown reply"},
		}},
	}, SendResult{
		MessageID:    "msg-1",
		DeliveryKind: "message.send",
		TargetType:   "group",
		TargetID:     "200",
	}, nil)

	summary := waitForOutboundSummary(t, stream)
	if summary.Message != "platform delivered group message: cooldown reply" {
		t.Fatalf("unexpected summary message: got %q", summary.Message)
	}
	if summary.PluginID != "" {
		t.Fatalf("unexpected plugin_id: %#v", summary.PluginID)
	}
	if _, ok := summary.Details["command_name"]; ok {
		t.Fatalf("unexpected command_name detail: %#v", summary.Details["command_name"])
	}
}

func TestLogSendOutcomeUsesPlatformFailureSummaryWithoutPluginContext(t *testing.T) {
	t.Parallel()

	logger, stream := newObservabilityTestLogger()

	LogSendOutcome(logger, SendLogContext{}, SendAttempt{
		ActionKind: "message.send",
		TargetType: "private",
		TargetID:   "300",
		Segments: []adapter.OutboundMessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "cooldown reply"},
		}},
	}, SendResult{
		DeliveryKind: "message.send",
		TargetType:   "private",
		TargetID:     "300",
	}, &adapter.Error{
		Code:    "adapter.send_failed",
		Message: "send rejected by upstream",
	})

	summary := waitForOutboundSummary(t, stream)
	if summary.Message != "platform failed to deliver private message: cooldown reply" {
		t.Fatalf("unexpected summary message: got %q", summary.Message)
	}
	if summary.Details["error_code"] != "adapter.send_failed" {
		t.Fatalf("unexpected error code: %#v", summary.Details["error_code"])
	}
}
