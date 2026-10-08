package qqofficial

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
)

func replyOrigin(targetType string, age time.Duration) chatevent.Event {
	return chatevent.Event{
		EventID: "event", SourceProtocol: SourceProtocol, SourceAdapter: "qq",
		EventType: "message." + targetType, MessageID: "inbound", Timestamp: time.Now().Add(-age).Unix(),
		Target: &chatevent.Target{Type: targetType, ID: "target"},
	}
}

func originSend(origin *chatevent.Event) chatevent.OutboundMessageSend {
	return chatevent.OutboundMessageSend{Origin: origin, TargetType: "group", TargetID: "target", Segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "answer"}}}}
}

func TestImplicitReplyUsesParentThroughHostRouting(t *testing.T) {
	client, captured := newTestClient(t, okResponse)
	client.adapterID = "qq"
	router := outbound.NewRouter(map[string]outbound.ActionSender{"qq": client}, map[string]string{"qq": SourceProtocol}, nil)
	var origin chatevent.Event
	client.SetEventHandler(func(_ context.Context, event chatevent.NormalizedEvent) { origin = chatevent.FromAdapter(event) })
	data, _ := json.Marshal(map[string]any{
		"id": "inbound", "group_openid": "target", "content": "question", "timestamp": time.Now().Unix(),
		"author": map[string]any{"id": "member"}, "mentions": []dispatchMention{{ID: "bot", IsYou: true}},
	})
	client.handleDispatch(t.Context(), gatewayFrame{T: dispatchGroupMessageCreate, D: data}, botProfile{})
	if origin.MessageID == "" {
		t.Fatal("full group message was not delivered")
	}
	for i := range 6 {
		_, err := outbound.SendAction(t.Context(), router, nil, origin, chatevent.MessageCommand{
			Kind: "message.send", TargetType: "group", TargetID: "target", MessageSegments: originSend(&origin).Segments,
		})
		if err != nil {
			t.Fatal(err)
		}
		body := (*captured)[i].body
		if i < 5 && (body.MsgID != origin.MessageID || body.MsgSeq != i+1) {
			t.Fatalf("reply %d=%+v", i, body)
		}
		if i == 5 && (body.MsgID != "" || body.MsgSeq != 0) {
			t.Fatalf("exhausted reply did not use active send: %+v", body)
		}
	}
}

func TestImplicitReplyWindowAndScope(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*chatevent.OutboundMessageSend)
		passive bool
	}{
		{name: "group current", passive: true},
		{name: "group expired", change: func(m *chatevent.OutboundMessageSend) { m.Origin.Timestamp = time.Now().Add(-6 * time.Minute).Unix() }},
		{name: "private within hour", passive: true, change: func(m *chatevent.OutboundMessageSend) {
			e := replyOrigin("private", 30*time.Minute)
			m.Origin = &e
			m.TargetType = "private"
		}},
		{name: "private expired", change: func(m *chatevent.OutboundMessageSend) {
			e := replyOrigin("private", 61*time.Minute)
			m.Origin = &e
			m.TargetType = "private"
		}},
		{name: "no parent", change: func(m *chatevent.OutboundMessageSend) { m.Origin = nil }},
		{name: "other adapter", change: func(m *chatevent.OutboundMessageSend) { m.Origin.SourceAdapter = "other" }},
		{name: "other conversation", change: func(m *chatevent.OutboundMessageSend) { m.TargetID = "other" }},
		{name: "other conversation kind", change: func(m *chatevent.OutboundMessageSend) { m.TargetType = "private" }},
		{name: "other protocol", change: func(m *chatevent.OutboundMessageSend) { m.Origin.SourceProtocol = "onebot11" }},
		{name: "notice", change: func(m *chatevent.OutboundMessageSend) { m.Origin.EventType = "notice.bot_added" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, captured := newTestClient(t, okResponse)
			client.adapterID = "qq"
			origin := replyOrigin("group", 0)
			message := originSend(&origin)
			if tc.change != nil {
				tc.change(&message)
			}
			if _, err := client.SendMessage(t.Context(), message); err != nil {
				t.Fatal(err)
			}
			if len(*captured) != 1 || ((*captured)[0].body.MsgID != "") != tc.passive {
				t.Fatalf("requests=%+v", *captured)
			}
		})
	}
}

func TestImplicitReplyFallsBackOnlyForKnownPassiveRefusals(t *testing.T) {
	for _, tc := range []struct {
		name         string
		status, code int
		fallback     bool
	}{
		{"expired", 400, 40004, true},
		{"platform expired", 400, 304027, true},
		{"quota", 400, 22009, true},
		{"invalid content", 400, 304003, false},
		{"rate limit", 429, 0, false},
		{"authentication", 401, 11244, false},
		{"uncertain receipt", 200, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempt := 0
			client, captured := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				attempt++
				if attempt > 1 {
					okResponse(w, r)
					return
				}
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"code": tc.code})
			})
			client.adapterID = "qq"
			origin := replyOrigin("group", 0)
			_, err := client.SendMessage(t.Context(), originSend(&origin))
			if (err == nil) != tc.fallback {
				t.Fatalf("err=%v", err)
			}
			want := 1
			if tc.fallback {
				want = 2
			}
			if len(*captured) != want || (*captured)[0].body.MsgID != "inbound" {
				t.Fatalf("requests=%+v", *captured)
			}
			if tc.fallback {
				if (*captured)[1].body.MsgID != "" || (*captured)[1].body.MsgSeq != 0 {
					t.Fatal("fallback still used passive reply")
				}
				if _, err := client.SendMessage(t.Context(), originSend(&origin)); err != nil {
					t.Fatal(err)
				}
				if (*captured)[2].body.MsgID != "" {
					t.Fatal("known unavailable reply was retried")
				}
			}
		})
	}
}

func TestConcurrentImplicitRepliesShareTheFiveReplyAllowance(t *testing.T) {
	sequences := newReplySequences()
	reply := passiveReply{messageID: "inbound", implicit: true, expiresAt: time.Now().Add(time.Minute)}
	results := make(chan int, 12)
	var wg sync.WaitGroup
	for range cap(results) {
		wg.Go(func() { results <- sequences.reserve(reply) })
	}
	wg.Wait()
	close(results)
	seen := make(map[int]bool)
	active := 0
	for seq := range results {
		if seq == 0 {
			active++
			continue
		}
		if seen[seq] {
			t.Fatalf("duplicate passive sequence %d", seq)
		}
		seen[seq] = true
	}
	if len(seen) != 5 || active != 7 {
		t.Fatalf("passive=%v active=%d", seen, active)
	}
}

func TestExplicitReplyDoesNotGainImplicitFallback(t *testing.T) {
	client, captured := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"code":40004}`))
	})
	_, err := client.SendReply(t.Context(), chatevent.OutboundMessageReply{TargetType: "group", TargetID: "target", ReplyToMessageID: "inbound", Segments: originSend(nil).Segments})
	if chatevent.SendErrorCode(err) != CodeReplyWindowExpired || len(*captured) != 1 {
		t.Fatalf("requests=%+v, err=%v", *captured, err)
	}
	client.adapterID = "qq"
	origin := replyOrigin("group", 0)
	message := originSend(&origin)
	message.Segments = append([]chatevent.MessageSegment{{Type: "reply", Data: map[string]any{"message_id": "inbound"}}}, message.Segments...)
	_, err = client.SendMessage(t.Context(), message)
	if chatevent.SendErrorCode(err) != CodeReplyWindowExpired || len(*captured) != 2 || (*captured)[1].body.MsgID != "inbound" {
		t.Fatalf("explicit reply segment lost its reference: requests=%+v, err=%v", *captured, err)
	}
}

func TestMultipartFallbackDoesNotRepeatDeliveredParts(t *testing.T) {
	sends := 0
	client, captured := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/groups/target/messages" {
			sends++
			if sends == 2 {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"code":22009}`))
				return
			}
		}
		okResponse(w, r)
	})
	client.adapterID = "qq"
	origin := replyOrigin("group", 0)
	message := originSend(&origin)
	message.Segments = append(message.Segments, chatevent.MessageSegment{Type: "image", Data: map[string]any{"url": "https://example.invalid/image.png"}})
	if _, err := client.SendMessage(t.Context(), message); err != nil {
		t.Fatal(err)
	}
	if len(*captured) != 4 || (*captured)[1].body.MsgType != msgTypeText || (*captured)[2].body.MsgType != msgTypeMedia || (*captured)[3].body.MsgType != msgTypeMedia || (*captured)[3].body.MsgID != "" {
		t.Fatalf("requests=%+v", *captured)
	}
}
