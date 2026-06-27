package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func dialProtectedWebSocket(t *testing.T, baseURL, path, token string) *websocket.Conn {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, response, err := websocket.Dial(ctx, websocketURL(baseURL)+path+"?session_token="+token, nil)
	if err != nil {
		if response == nil {
			t.Fatalf("dial websocket: %v", err)
		}
		t.Fatalf("dial websocket returned status %d: %v", response.StatusCode, err)
	}

	return conn
}

func readWebSocketJSON(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()

	readCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, payload, err := conn.Read(readCtx)
	if err != nil {
		t.Fatalf("read websocket frame: %v", err)
	}

	var frame map[string]any
	if err := json.Unmarshal(payload, &frame); err != nil {
		t.Fatalf("unmarshal websocket frame: %v", err)
	}

	return frame
}
