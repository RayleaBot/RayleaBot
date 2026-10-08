package qqofficial

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
)

func TestFullGroupMessagesRequireThisBotMention(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		mentions      []dispatchMention
		want          bool
		text          string
	}{
		{name: "plain", content: "hello"},
		{name: "other bot", content: "<@other> hello", mentions: []dispatchMention{{ID: "other"}}},
		{name: "is you", content: "<@alias> hello", mentions: []dispatchMention{{ID: "alias", IsYou: true}}, want: true, text: "hello"},
		{name: "identity", content: "hello", mentions: []dispatchMention{{ID: "bot"}}, want: true, text: "hello"},
		{name: "at", content: "<@bot> hello <@other>", want: true, text: "hello <@other>"},
		{name: "nickname at", content: "<@!bot> hello", want: true, text: "hello"},
		{name: "text chain", content: `<qqbot-at-user id="bot" /> hello`, want: true, text: "hello"},
		{name: "mention only", content: "<@bot>", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, _ := json.Marshal(dispatchMessage{ID: "message", GroupOpenID: "group", Author: dispatchAuthor{MemberOpenID: "member"}, Content: tc.content, Mentions: tc.mentions})
			event, ok := NormalizeDispatch("event", dispatchGroupMessageCreate, data, "bot")
			if ok != tc.want {
				t.Fatalf("delivered=%v, want %v", ok, tc.want)
			}
			if !ok {
				return
			}
			if event.PlainText != tc.text || event.EventType != "message.group" || event.ConversationID != "group" || event.MessageID != "message" {
				t.Fatalf("event=%+v", event)
			}
			if event.PayloadFields["qq_official"].(map[string]any)["dispatch_type"] != dispatchGroupMessageCreate {
				t.Fatal("lost native dispatch type")
			}
			if tc.text == "" && len(event.Segments) != 0 || tc.text != "" && (len(event.Segments) != 1 || event.Segments[0].Data["text"] != tc.text) {
				t.Fatalf("segments=%+v", event.Segments)
			}
		})
	}
}

func TestGroupDispatchesDeduplicatePerInstanceConcurrently(t *testing.T) {
	for _, first := range []string{dispatchGroupAtMessageCreate, dispatchGroupMessageCreate} {
		t.Run(first, func(t *testing.T) {
			var delivered atomic.Int32
			client := &Client{adapterID: "qq", logger: discardLogger()}
			client.SetEventHandler(func(_ context.Context, _ chatevent.NormalizedEvent) { delivered.Add(1) })
			data := json.RawMessage(`{"id":"same","group_openid":"group","author":{"id":"member"},"content":"hi","mentions":[{"id":"bot","is_you":true}]}`)
			client.handleDispatch(t.Context(), gatewayFrame{T: first, D: data}, botProfile{})
			var wg sync.WaitGroup
			for i := range 20 {
				wg.Go(func() {
					kind := dispatchGroupAtMessageCreate
					if i%2 == 0 {
						kind = dispatchGroupMessageCreate
					}
					client.handleDispatch(t.Context(), gatewayFrame{T: kind, D: data}, botProfile{})
				})
			}
			wg.Wait()
			if delivered.Load() != 1 {
				t.Fatalf("delivered=%d", delivered.Load())
			}
			other := &Client{adapterID: "other", logger: discardLogger()}
			other.SetEventHandler(client.eventHandler())
			other.handleDispatch(t.Context(), gatewayFrame{T: first, D: data}, botProfile{})
			if delivered.Load() != 2 {
				t.Fatal("different instance lost its message")
			}
		})
	}
}
