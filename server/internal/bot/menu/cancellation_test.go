package menu

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestCanceledMenuRenderDoesNotSendFallback(t *testing.T) {
	var logs bytes.Buffer
	sender := &adapterReplySender{}
	called := false
	s := New(Deps{
		CurrentConfig: func() config.Config {
			return config.Config{Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Commands: []string{"帮助"}, Prefixes: []string{"#"}}}}
		},
		Sender: sender, Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
		Renderer: func(context.Context, RenderRequest) (string, error) {
			called = true
			return "", fmt.Errorf("render: %w", context.Canceled)
		},
	})
	event := chatevent.NormalizedEvent{ConversationType: "private", ConversationID: "10002", SenderID: "10002", MessageID: "1", Segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "#帮助"}}}}
	if !s.Handle(t.Context(), event) || !called {
		t.Fatal("menu renderer was not invoked")
	}
	if sender.sent.TargetID != "" || sender.reply.ReplyToMessageID != "" || strings.Contains(logs.String(), `"level":"WARN"`) {
		t.Fatalf("cancelled menu took failure fallback: %s", logs.String())
	}
}
