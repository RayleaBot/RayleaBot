package integration

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/testutil"
	"github.com/coder/websocket"
)

func dialEventsWebSocket(t *testing.T, baseURL, token string) *websocket.Conn {
	return testutil.DialEventsWebSocket(t, baseURL, token)
}

func testBridgeEvent() adapterintake.NormalizedEvent {
	return adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessageText,
		EventID:          "onebot11-message-1001",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Unix(1_700_000_123, 0).Unix(),
		ConversationType: "group",
		ConversationID:   "2001",
		SenderID:         "3001",
		PlainText:        "hello bridge",
	}
}

func waitForObservabilitySubscriber(t *testing.T, eventBridge *bridge.Bridge) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if eventBridge.ObservabilitySubscriberCount() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for websocket subscriber")
}

func newTestAppWithOneBotAccessToken(t *testing.T, accessToken string, authOptions ...auth.Option) *app.App {
	t.Helper()

	fixture := loadConfigFixture(t, filepath.Join("..", "fixtures", "config", "ok.minimal.json"))

	var input map[string]any
	if err := json.Unmarshal(fixture.Input, &input); err != nil {
		t.Fatalf("unmarshal config fixture input: %v", err)
	}

	onebot := input["onebot"].(map[string]any)
	onebot["forward_ws"].(map[string]any)["access_token"] = accessToken

	updated, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal config fixture input: %v", err)
	}

	configPath := writeYAMLConfig(t, updated)
	schemaPath := filepath.Join("..", "contracts", "config.user.schema.json")

	application, err := app.New(app.Options{
		ConfigPath:  configPath,
		SchemaPath:  schemaPath,
		AuthOptions: authOptions,
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})
	return application
}
