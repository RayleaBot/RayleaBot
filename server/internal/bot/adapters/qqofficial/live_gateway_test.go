//go:build manual_smoke

package qqofficial

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

// TestLiveGatewayHandshake drives the real client against the QQ Open Platform.
// Select it explicitly with the manual_smoke build tag and provide credentials
// through the environment; see docs/engineering/manual-smoke.md:
//
//	QQ_APP_ID=... QQ_APP_SECRET=... go test -tags manual_smoke ./internal/bot/adapters/qqofficial/ -run LiveGateway
func TestLiveGatewayHandshake(t *testing.T) {
	appID, appSecret := os.Getenv("QQ_APP_ID"), os.Getenv("QQ_APP_SECRET")
	if appID == "" || appSecret == "" {
		t.Fatal("set QQ_APP_ID and QQ_APP_SECRET to exercise the live gateway")
	}

	client := New("qq-official",
		config.QQOfficialConfig{
			AppID: appID, AppSecret: appSecret,
			Intents: []string{"group_and_c2c", "public_guild_messages"},
		},
		config.AdapterConfig{ConnectTimeoutSeconds: 15, ReconnectInitialSeconds: 2, ReconnectMultiplier: 2, ReconnectMaxSeconds: 30},
		discardLogger(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client.Start(ctx)
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		if err := client.Stop(stopCtx); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	}()

	// READY is the gateway's confirmation that identify succeeded; the bot
	// identity only exists once it arrives.
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if id, name := client.BotIdentity(); id != "" {
			t.Logf("gateway handshake complete: bot_id=%s bot_name=%s", id, name)
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("gateway did not reach READY within the deadline")
}
