package qqofficial

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

// TestLiveMediaReply answers the next inbound message with a local image,
// exercising the real upload-then-send path against the platform. It replies
// rather than pushes, so it does not spend the active message quota.
//
// Skipped unless credentials and an image are supplied:
//
//	QQ_APP_ID=... QQ_APP_SECRET=... QQ_LIVE_IMAGE=/path/to.png \
//	  go test ./internal/qqofficial/ -run LiveMediaReply -v -timeout 5m
func TestLiveMediaReply(t *testing.T) {
	appID, appSecret := os.Getenv("QQ_APP_ID"), os.Getenv("QQ_APP_SECRET")
	imagePath := os.Getenv("QQ_LIVE_IMAGE")
	if appID == "" || appSecret == "" || imagePath == "" {
		t.Skip("set QQ_APP_ID, QQ_APP_SECRET and QQ_LIVE_IMAGE to exercise a live media reply")
	}
	if _, err := os.Stat(imagePath); err != nil {
		t.Fatalf("QQ_LIVE_IMAGE is not readable: %v", err)
	}

	client := New(
		config.QQOfficialConfig{
			Enabled: true, AppID: appID, AppSecret: appSecret,
			Intents: []string{"group_and_c2c", "public_guild_messages"},
		},
		config.AdapterConfig{ConnectTimeoutSeconds: 15, ReconnectInitialSeconds: 2, ReconnectMultiplier: 2, ReconnectMaxSeconds: 30},
		discardLogger(),
	)

	var once sync.Once
	received := make(chan chatevent.NormalizedEvent, 1)
	client.SetEventHandler(func(_ context.Context, event chatevent.NormalizedEvent) {
		once.Do(func() { received <- event })
	})

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	client.Start(ctx)
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		_ = client.Stop(stopCtx)
	}()

	t.Log("connected; waiting for an inbound message to answer")
	var event chatevent.NormalizedEvent
	select {
	case event = <-received:
	case <-ctx.Done():
		t.Fatal("no inbound message arrived within the window")
	}
	t.Logf("inbound: type=%s conversation=%s/%s message_id=%s",
		event.EventType, event.ConversationType, event.ConversationID, event.MessageID)

	sendCtx, sendCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer sendCancel()
	result, err := client.SendReply(sendCtx, chatevent.OutboundMessageReply{
		SourceProtocol:   SourceProtocol,
		TargetType:       event.ConversationType,
		TargetID:         event.ConversationID,
		ReplyToMessageID: event.MessageID,
		Segments: []chatevent.MessageSegment{
			{Type: "text", Data: map[string]any{"text": "RayleaBot 富媒体自检"}},
			{Type: "image", Data: map[string]any{"file": imagePath}},
		},
	})
	if err != nil {
		t.Fatalf("live media reply failed: %v", err)
	}
	t.Logf("reply delivered: message_id=%s", result.MessageID)
}
