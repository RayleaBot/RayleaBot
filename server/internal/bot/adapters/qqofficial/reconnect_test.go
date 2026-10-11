package qqofficial

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters/reconnect"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/coder/websocket"
)

type reconnectTestConn struct {
	profileTestConn
	resumed   bool
	requested bool
}

func (c *reconnectTestConn) Read(ctx context.Context) (websocket.MessageType, []byte, error) {
	if c.reads == 1 && c.resumed {
		c.reads++
		return websocket.MessageText, []byte(`{"op":0,"t":"RESUMED","d":{}}`), nil
	}
	if c.reads == 2 && c.requested {
		c.reads++
		return websocket.MessageText, []byte(`{"op":7}`), nil
	}
	return c.profileTestConn.Read(ctx)
}

func TestReconnectBackoffResetsAfterReadyAndResumed(t *testing.T) {
	for _, requested := range []bool{false, true} {
		t.Run(map[bool]string{false: "transport loss", true: "gateway reconnect"}[requested], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := New("qq", config.QQOfficialConfig{AppID: "10001", AppSecret: "fixture"}, config.AdapterConfig{}, discardLogger())
				c.tokens.token, c.tokens.expiresAt = "fixture", time.Now().Add(time.Hour)
				c.backoff = reconnect.NewWithDurations(time.Second, 2, 16*time.Second, 0, nil)
				c.http.Transport = reloadTestTransport(func(r *http.Request) (*http.Response, error) {
					body := `{"url":"wss://example.invalid/gateway"}`
					if r.URL.Path == "/users/@me" {
						body = `{"id":"bot-1","username":"fixture","avatar":"https://example.invalid/avatar.png"}`
					}
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
				})
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				start := time.Now()
				var attempts []time.Duration
				c.dialer = func(context.Context, string) (wsConn, error) {
					attempts = append(attempts, time.Since(start))
					switch len(attempts) {
					case 1, 2:
						return nil, errors.New("fixture dial failure")
					case 3, 4:
						return &reconnectTestConn{resumed: len(attempts) == 4, requested: requested}, nil
					default:
						cancel()
						return nil, context.Canceled
					}
				}
				c.Start(ctx)
				<-c.done
				want := []time.Duration{0, time.Second, 3 * time.Second, 4 * time.Second, 5 * time.Second}
				if len(attempts) != len(want) {
					t.Fatalf("attempts = %v", attempts)
				}
				for i := range want {
					if attempts[i] != want[i] {
						t.Fatalf("attempts = %v, want %v", attempts, want)
					}
				}
			})
		})
	}
}
