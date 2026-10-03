package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
)

type countedSendFailure struct{ formatted *int }

func (failure countedSendFailure) Error() string {
	(*failure.formatted)++
	return "fixture failure"
}

func TestSendLoggingFollowsLevelChangesWithoutFormattingFilteredFailures(t *testing.T) {
	var output bytes.Buffer
	var level slog.LevelVar
	level.Set(slog.LevelError)
	logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: &level})).WithGroup("send").With("target_id", "shadowed")
	attempt := SendAttempt{ActionKind: "message.send", TargetType: "group", TargetID: "200", Segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "fixture"}}}}
	formatted := 0
	failure := countedSendFailure{formatted: &formatted}
	LogSendOutcome(logger, SendLogContext{}, attempt, SendResult{}, failure)
	if output.Len() != 0 || formatted != 0 {
		t.Fatalf("filtered failure was formatted: output=%q calls=%d", output.String(), formatted)
	}
	level.Set(slog.LevelWarn)
	LogSendOutcome(logger, SendLogContext{}, attempt, SendResult{}, nil)
	if output.Len() != 0 {
		t.Fatal("INFO send log escaped the WARN threshold")
	}
	LogSendOutcome(logger, SendLogContext{}, attempt, SendResult{}, failure)
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	details := record["send"].(map[string]any)
	if record["level"] != "WARN" || details["target_id"] != "200" || details["reason"] != "fixture failure" || formatted != 1 {
		t.Fatalf("enabled failure log changed: %s, formatted=%d", output.Bytes(), formatted)
	}
	output.Reset()
	level.Set(slog.LevelInfo)
	LogSendOutcome(logger, SendLogContext{}, attempt, SendResult{}, nil)
	if err := json.Unmarshal(output.Bytes(), &record); err != nil || record["level"] != "INFO" {
		t.Fatalf("INFO logging was not restored: %s, %v", output.Bytes(), err)
	}
}

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

type stubTargetDisplayResolver map[string]string

func (s stubTargetDisplayResolver) ResolveTargetName(_ context.Context, adapterID, targetType, targetID string) string {
	return s[adapterID+"/"+targetType+":"+targetID]
}

func TestLogSendOutcomeUsesPlatformSummaryWithoutPluginContext(t *testing.T) {
	t.Parallel()

	logger, stream := newObservabilityTestLogger()

	LogSendOutcome(logger, SendLogContext{}, SendAttempt{
		ActionKind: "message.send",
		TargetType: "group",
		TargetID:   "200",
		Segments: []chatevent.MessageSegment{{
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
	if summary.Details["outcome"] != "delivered" || summary.Details["plain_text"] != "cooldown reply" || summary.Details["target_id"] != "200" {
		t.Fatalf("unexpected summary message: got %q", summary.Message)
	}
	// Without a caller-built label the body still names the conversation
	// from the attempt, so the line never reads as a bare "sent".
	if summary.Message != "消息已发送：[200]: cooldown reply" {
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
		Segments: []chatevent.MessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "cooldown reply"},
		}},
	}, SendResult{
		DeliveryKind: "message.send",
		TargetType:   "private",
		TargetID:     "300",
	}, &chatevent.SendError{
		Code:    "adapter.send_failed",
		Message: "send rejected by upstream",
	})

	summary := waitForOutboundSummary(t, stream)
	if summary.Details["outcome"] != "failed" || summary.Details["reason"] != "send rejected by upstream" || summary.Details["target_id"] != "300" {
		t.Fatalf("unexpected summary message: got %q", summary.Message)
	}
	if summary.Details["error_code"] != "adapter.send_failed" {
		t.Fatalf("unexpected error code: %#v", summary.Details["error_code"])
	}
	if summary.Message != "消息发送失败：私聊(300): cooldown reply" {
		t.Fatalf("unexpected summary message: got %q", summary.Message)
	}
}

func TestLogSendOutcomeNamesBotAccountAndTarget(t *testing.T) {
	t.Parallel()

	logger, stream := newObservabilityTestLogger()

	LogSendOutcome(logger, SendLogContext{
		PluginID:    "weather",
		BotID:       "10001",
		BotNickname: "测试机器人\u202e",
		TargetLabel: "[测试群(200)]",
	}, SendAttempt{
		ActionKind: "message.send",
		TargetType: "group",
		TargetID:   "200",
		Segments: []chatevent.MessageSegment{{
			Type: "text",
			Data: map[string]any{"text": "  hello world  "},
		}},
	}, SendResult{
		MessageID:    "msg-1",
		DeliveryKind: "message.send",
		TargetType:   "group",
		TargetID:     "200",
	}, nil)

	summary := waitForOutboundSummary(t, stream)
	if summary.Message != "消息已发送：测试机器人(10001) -> [测试群(200)]: hello world" {
		t.Fatalf("unexpected summary message: got %q", summary.Message)
	}
	if summary.Details["self_id"] != "10001" || summary.Details["self_nickname"] != "测试机器人" {
		t.Fatalf("unexpected bot details: self_id=%#v self_nickname=%#v", summary.Details["self_id"], summary.Details["self_nickname"])
	}
	if summary.Details["target_label"] != "[测试群(200)]" {
		t.Fatalf("unexpected target_label detail: %#v", summary.Details["target_label"])
	}
}

func TestLogSendOutcomeMarksUnconfirmedDeliveryWithTheSameBody(t *testing.T) {
	t.Parallel()

	logger, stream := newObservabilityTestLogger()

	LogSendOutcome(logger, SendLogContext{
		BotID:       "10001",
		TargetLabel: "测试用户A(300)",
	}, SendAttempt{
		ActionKind: "message.send",
		TargetType: "private",
		TargetID:   "300",
		Segments: []chatevent.MessageSegment{{
			Type: "image",
			Data: map[string]any{"file": "https://example.com/a.png"},
		}},
	}, SendResult{
		DeliveryKind: "message.send",
		TargetType:   "private",
		TargetID:     "300",
	}, &chatevent.SendError{
		Code:    errorcodes.AdapterSendUnconfirmed,
		Message: "no receipt before the deadline",
	})

	summary := waitForOutboundSummary(t, stream)
	if summary.Level != "warn" {
		t.Fatalf("unexpected level: got %q want warn", summary.Level)
	}
	if summary.Message != "消息发送状态未确认，不自动重发：10001 -> 测试用户A(300): [图片]" {
		t.Fatalf("unexpected summary message: got %q", summary.Message)
	}
	if summary.Details["outcome"] != "unconfirmed" || summary.Details["error_code"] != errorcodes.AdapterSendUnconfirmed {
		t.Fatalf("unexpected unconfirmed details: %#v", summary.Details)
	}
}

type stubBotDisplayResolver map[string][2]string

func (s stubBotDisplayResolver) ResolveBotDisplay(adapterID string) (string, string) {
	identity := s[adapterID]
	return identity[0], identity[1]
}

func TestResolveBotIdentityPrefersTheAdapterLoginOverTheEvent(t *testing.T) {
	t.Parallel()

	resolver := stubBotDisplayResolver{"bot-one": {"10001", "测试机器人"}}

	if id, nickname := ResolveBotIdentity("bot-one", "99999", "", resolver); id != "10001" || nickname != "测试机器人" {
		t.Fatalf("unexpected identity from the adapter: got %q/%q", id, nickname)
	}
	// An adapter that cannot answer leaves the identity of the event in place.
	if id, nickname := ResolveBotIdentity("bot-two", "99999", "事件机器人", resolver); id != "99999" || nickname != "事件机器人" {
		t.Fatalf("unexpected fallback identity: got %q/%q", id, nickname)
	}
	if id, nickname := ResolveBotIdentity("", "99999", "", nil); id != "99999" || nickname != "" {
		t.Fatalf("unexpected identity without a resolver: got %q/%q", id, nickname)
	}
}

func TestBuildTargetLabelPrefersEventContextForPrivateMessage(t *testing.T) {
	t.Parallel()

	label := BuildTargetLabel(context.Background(), "onebot11", "private", "300", "", "300", "测试用户A", stubTargetDisplayResolver{
		"onebot11/private:300": "测试用户B",
	})
	if label != "测试用户A(300)" {
		t.Fatalf("unexpected private label: got %q want %q", label, "测试用户A(300)")
	}
}

func TestBuildTargetLabelUsesResolverAndFallbackFormats(t *testing.T) {
	t.Parallel()

	groupLabel := BuildTargetLabel(context.Background(), "onebot11", "group", "200", "", "", "", stubTargetDisplayResolver{
		"onebot11/group:200": "测试群",
	})
	if groupLabel != "[测试群(200)]" {
		t.Fatalf("unexpected group label: got %q want %q", groupLabel, "[测试群(200)]")
	}

	privateLabel := BuildTargetLabel(context.Background(), "onebot11", "private", "300", "", "", "", stubTargetDisplayResolver{})
	if privateLabel != "私聊(300)" {
		t.Fatalf("unexpected private fallback label: got %q want %q", privateLabel, "私聊(300)")
	}
}
