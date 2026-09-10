package qqofficial

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/errorcodes"
)

func TestPostWithoutReceiptIsUnconfirmedAndSentOnlyOnce(t *testing.T) {
	client, captured := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		connection, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		_ = connection.Close()
	})
	_, err := client.SendMessage(context.Background(), chatevent.OutboundMessageSend{TargetType: "group", TargetID: "group", Segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "one message"}}}})
	var failure *chatevent.SendError
	if !errors.As(err, &failure) || failure.Code != errorcodes.AdapterSendUnconfirmed || failure.Err == nil {
		t.Fatalf("uncertain send=%v", err)
	}
	if len(*captured) != 1 {
		t.Fatalf("send count=%d", len(*captured))
	}
}

func TestUpstreamAuthenticationFailureHasNeutralClassification(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) })
	_, err := client.SendMessage(context.Background(), chatevent.OutboundMessageSend{TargetType: "private", TargetID: "user", Segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "message"}}}})
	if chatevent.SendErrorCode(err) != errorcodes.AdapterAuthFailed || chatevent.SendOutcome(err) != "permission_denied" {
		t.Fatalf("authentication error=%v", err)
	}
}
