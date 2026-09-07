package qqofficial

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
)

// The QQ adapter must be usable through the same seam as the OneBot adapter;
// that is the whole point of the neutral outbound types.
var _ outbound.ActionSender = (*Client)(nil)

type capturedRequest struct {
	path string
	body sendMessageRequest
}

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *[]capturedRequest) {
	t.Helper()
	captured := &[]capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body sendMessageRequest
		json.NewDecoder(r.Body).Decode(&body)
		*captured = append(*captured, capturedRequest{path: r.URL.Path, body: body})
		handler(w, r)
	}))
	t.Cleanup(server.Close)

	tokens := NewTokenSource("app", "secret", server.Client())
	tokens.endpoint = server.URL + "/token"
	// The token endpoint shares the stub, so hand back a canned token.
	client := &Client{
		appID: "app", apiBase: server.URL, http: server.Client(),
		tokens: tokens, replies: newReplySequences(), logger: discardLogger(),
	}
	tokens.token = "canned-token"
	tokens.expiresAt = tokens.now().Add(3600 * 1e9)
	return client, captured
}

func okResponse(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/files") {
		json.NewEncoder(w).Encode(map[string]any{"file_uuid": "u-1", "file_info": "file-info-1", "ttl": 600})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{"id": "sent-1"})
}

func TestSendRoutesConversationsToTheirEndpoints(t *testing.T) {
	t.Parallel()

	client, captured := newTestClient(t, okResponse)
	text := []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "hello"}}}

	if _, err := client.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1", Segments: text,
	}); err != nil {
		t.Fatalf("SendMessage(group): %v", err)
	}
	if _, err := client.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "private", TargetID: "U1", Segments: text,
	}); err != nil {
		t.Fatalf("SendMessage(private): %v", err)
	}
	if got := (*captured)[0].path; got != "/v2/groups/G1/messages" {
		t.Fatalf("group endpoint = %q", got)
	}
	if got := (*captured)[1].path; got != "/v2/users/U1/messages" {
		t.Fatalf("private endpoint = %q", got)
	}
	// An active push carries no msg_id; that is what distinguishes it from a
	// passive reply for quota purposes.
	if (*captured)[0].body.MsgID != "" {
		t.Fatalf("active send carried msg_id %q", (*captured)[0].body.MsgID)
	}
}

func TestReplyCarriesTheInboundMessageAndAdvancesSequence(t *testing.T) {
	t.Parallel()

	client, captured := newTestClient(t, okResponse)
	reply := chatevent.OutboundMessageReply{
		TargetType: "group", TargetID: "G1", ReplyToMessageID: "ROBOT1.0_abc",
		Segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "pong"}}},
	}
	for i := 0; i < 2; i++ {
		if _, err := client.SendReply(context.Background(), reply); err != nil {
			t.Fatalf("SendReply %d: %v", i, err)
		}
	}
	if (*captured)[0].body.MsgID != "ROBOT1.0_abc" {
		t.Fatalf("reply msg_id = %q, want the inbound message id", (*captured)[0].body.MsgID)
	}
	// The platform rejects a repeated msg_seq against the same msg_id, so two
	// answers to one message must not share a sequence.
	if (*captured)[0].body.MsgSeq == (*captured)[1].body.MsgSeq {
		t.Fatalf("both replies used msg_seq %d", (*captured)[0].body.MsgSeq)
	}
}

func TestSendRejectsUnaddressableConversations(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, okResponse)
	text := []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "hi"}}}
	for _, target := range []struct{ kind, id string }{
		{"channel", "C1"}, // no channel support until guild events are delivered
		{"group", ""},
	} {
		_, err := client.SendMessage(context.Background(), chatevent.OutboundMessageSend{
			TargetType: target.kind, TargetID: target.id, Segments: text,
		})
		if err == nil {
			t.Fatalf("target %q/%q was accepted", target.kind, target.id)
		}
	}
}

func TestSendReportsSegmentsItCannotDeliver(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, okResponse)
	// The platform has media kinds for image, video and voice only; anything
	// else must fail loudly rather than send nothing.
	_, err := client.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1",
		Segments: []chatevent.MessageSegment{{Type: "poke", Data: map[string]any{}}},
	})
	if err == nil || !strings.Contains(err.Error(), "poke") {
		t.Fatalf("error = %v, want it to name the undeliverable segment kind", err)
	}
}

func TestSendSurfacesPlatformRejection(t *testing.T) {
	t.Parallel()

	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{"code": 40034, "message": "push message is limited"})
	})
	_, err := client.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1",
		Segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "hi"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "push message is limited") {
		t.Fatalf("error = %v, want the platform's own reason", err)
	}
}

func TestReplySequencesAreScopedPerMessage(t *testing.T) {
	t.Parallel()

	sequences := newReplySequences()
	if first, second := sequences.next("a"), sequences.next("a"); first == second {
		t.Fatalf("same message reused msg_seq %d", first)
	}
	// Sequences are per inbound message, so a different message starts over.
	if got := sequences.next("b"); got != 1 {
		t.Fatalf("new message started at msg_seq %d, want 1", got)
	}
}

func TestSendErrorsCarryFormalCodes(t *testing.T) {
	t.Parallel()

	text := []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "hi"}}}

	// A quota refusal is retryable later; an expired reply window is not. A
	// caller that cannot tell them apart will either retry forever or give up.
	quota, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{"code": 40034, "message": "push message is limited"})
	})
	_, err := quota.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1", Segments: text,
	})
	var sendErr *SendError
	if !errors.As(err, &sendErr) || sendErr.Code != CodeMessageQuotaExceeded {
		t.Fatalf("quota refusal = %v, want %s", err, CodeMessageQuotaExceeded)
	}
	if sendErr.Message != "push message is limited" {
		t.Fatalf("message = %q, want the platform's own wording", sendErr.Message)
	}

	window, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"code": 40004, "message": "msg_id expired"})
	})
	_, err = window.SendReply(context.Background(), chatevent.OutboundMessageReply{
		TargetType: "group", TargetID: "G1", ReplyToMessageID: "ROBOT1.0_abc", Segments: text,
	})
	if !errors.As(err, &sendErr) || sendErr.Code != CodeReplyWindowExpired {
		t.Fatalf("refused reply = %v, want %s", err, CodeReplyWindowExpired)
	}

	// The same status on an active push is not a reply-window problem.
	active, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"code": 40004, "message": "bad request"})
	})
	_, err = active.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1", Segments: text,
	})
	if !errors.As(err, &sendErr) || sendErr.Code != CodeSendFailed {
		t.Fatalf("refused active push = %v, want %s", err, CodeSendFailed)
	}

	unsupported, _ := newTestClient(t, okResponse)
	_, err = unsupported.SendMessage(context.Background(), chatevent.OutboundMessageSend{
		TargetType: "group", TargetID: "G1",
		Segments: []chatevent.MessageSegment{{Type: "poke", Data: map[string]any{}}},
	})
	if !errors.As(err, &sendErr) || sendErr.Code != CodeCapabilityUnsupported {
		t.Fatalf("undeliverable segment = %v, want %s", err, CodeCapabilityUnsupported)
	}
}
