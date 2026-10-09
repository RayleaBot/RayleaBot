package qqofficial

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/coder/websocket"
)

func TestFetchBotProfile(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		status int
		body   string
		want   botProfile
	}{
		{"profile", 200, `{"id":"bot-1","username":"机器人","avatar":"https://example.com/bot.png"}`, botProfile{"bot-1", "机器人", "https://example.com/bot.png"}},
		{"http QQ avatar", 200, `{"id":"bot-1","avatar":"http://thirdqq.qlogo.cn/g?b=oidb&k=fixture&s=100"}`, botProfile{ID: "bot-1", AvatarURL: "https://thirdqq.qlogo.cn/g?b=oidb&k=fixture&s=100"}},
		{"http qpic avatar", 200, `{"id":"bot-1","avatar":"http://q.qpic.cn/fixture"}`, botProfile{ID: "bot-1", AvatarURL: "https://q.qpic.cn/fixture"}},
		{"missing avatar", 200, `{"id":"bot-1","username":"机器人"}`, botProfile{"bot-1", "机器人", ""}},
		{"rejected", 403, `{"id":"wrong","username":"wrong"}`, botProfile{}},
		{"malformed", 200, `{`, botProfile{}},
		{"oversized", 200, `{"id":"bot-1","username":"` + strings.Repeat("a", 64<<10) + `"}`, botProfile{}},
		{"insecure avatar", 200, `{"id":"bot-1","avatar":"http://example.com/bot.png"}`, botProfile{ID: "bot-1"}},
		{"credential in avatar", 200, `{"id":"bot-1","avatar":"https://user:pass@example.com/bot.png"}`, botProfile{ID: "bot-1"}},
		{"invalid avatar", 200, `{"id":"bot-1","avatar":"https://%"}`, botProfile{ID: "bot-1"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/users/@me" || r.Header.Get("Authorization") != "QQBot fixture-token" || r.Header.Get("X-Union-Appid") != "10001" {
					t.Error("profile request did not use the connection credentials and endpoint")
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer srv.Close()
			got, err := fetchBotProfile(context.Background(), srv.Client(), srv.URL, "10001", "fixture-token")
			if got != tt.want {
				t.Fatalf("profile = %+v, want %+v", got, tt.want)
			}
			if (err != nil) != (tt.want.AvatarURL == "") {
				t.Fatalf("profile error = %v, want failure = %v", err, tt.want.AvatarURL == "")
			}
		})
	}
}

func TestFetchBotProfileBoundsWait(t *testing.T) {
	t.Parallel()
	called := false
	client := &http.Client{Transport: reloadTestTransport(func(r *http.Request) (*http.Response, error) {
		called = true
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Error("optional profile request has no bounded deadline")
		}
		return nil, context.DeadlineExceeded
	})}
	if _, err := fetchBotProfile(context.Background(), client, "https://example.com", "10001", "fixture"); err == nil {
		t.Fatal("profile timeout was not reported")
	}
	if !called {
		t.Fatal("profile request was not sent")
	}
}

func TestReadyProfileMatchesIdentityAndReloadClearsIt(t *testing.T) {
	t.Parallel()
	ready := gatewayFrame{Op: opDispatch, T: dispatchReady, D: json.RawMessage(`{"session_id":"s1","user":{"id":"bot-1","username":"gateway name"}}`)}
	for _, profile := range []botProfile{{}, {ID: "other-bot", Name: "other", AvatarURL: "https://example.com/other.png"}, {ID: "bot-1", Name: "profile name", AvatarURL: "https://example.com/bot.png"}} {
		c := New("qq", config.QQOfficialConfig{AppID: "10001", AppSecret: "fixture-secret"}, config.AdapterConfig{}, discardLogger())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c.setConnectionCancel(cancel)
		c.handleDispatch(ctx, ready, profile)
		got := c.Status()
		wantName, wantAvatar := "gateway name", ""
		if profile.ID == "bot-1" {
			wantName, wantAvatar = profile.Name, profile.AvatarURL
		}
		if got.State != StateConnected || got.BotID != "bot-1" || got.BotName != wantName || got.BotAvatarURL != wantAvatar {
			t.Fatalf("READY profile mixed identities: %+v", got)
		}
		c.Reload(config.QQOfficialConfig{AppID: "20002", AppSecret: "fixture-next"})
		// Late profile/READY/RESUMED frames from the replaced connection cannot
		// restore the previous account, even before the next connection starts.
		c.handleDispatch(ctx, ready, profile)
		c.handleDispatch(ctx, gatewayFrame{Op: opDispatch, T: dispatchResumed}, profile)
		if got := c.Status(); got.BotID != "" || got.BotName != "" || got.BotAvatarURL != "" {
			t.Fatalf("reloaded client retained old identity: %+v", got)
		}
	}
}

func TestResumedRefreshesOnlyItsOwnProfileAndDisabledHidesIdentity(t *testing.T) {
	t.Parallel()
	c := &Client{logger: discardLogger()}
	c.session.startSession("s1", "bot-1", "old", "https://example.com/old.png")
	frame := gatewayFrame{Op: opDispatch, T: dispatchResumed}
	c.handleDispatch(context.Background(), frame, botProfile{ID: "other", Name: "wrong", AvatarURL: "https://example.com/wrong.png"})
	if got := c.Status(); got.BotName != "old" || got.BotAvatarURL != "https://example.com/old.png" {
		t.Fatalf("mismatched resume profile: %+v", got)
	}
	c.handleDispatch(context.Background(), frame, botProfile{ID: "bot-1", Name: "new", AvatarURL: "https://example.com/new.png"})
	if got := c.Status(); got.BotName != "new" || got.BotAvatarURL != "https://example.com/new.png" {
		t.Fatalf("profile did not refresh: %+v", got)
	}
	c.SetEnabled(false)
	if got := c.Status(); got.State != StateStopped || got.BotID != "" || got.BotAvatarURL != "" {
		t.Fatalf("disabled profile: %+v", got)
	}
}

func TestReadyRetainsAvatarForTheSameBot(t *testing.T) {
	t.Parallel()
	c := &Client{logger: discardLogger()}
	c.session.startSession("old-session", "bot-1", "old", "https://thirdqq.qlogo.cn/fixture")
	ready := gatewayFrame{Op: opDispatch, T: dispatchReady, D: json.RawMessage(`{"session_id":"new-session","user":{"id":"bot-1","username":"gateway bot"}}`)}
	c.handleDispatch(t.Context(), ready, botProfile{})
	c.handleDispatch(t.Context(), gatewayFrame{Op: opDispatch, T: dispatchResumed}, botProfile{})
	if got := c.Status(); got.State != StateConnected || got.BotAvatarURL != "https://thirdqq.qlogo.cn/fixture" {
		t.Fatalf("reconnection lost avatar after profile failure: %+v", got)
	}
	ready.D = json.RawMessage(`{"session_id":"other-session","user":{"id":"bot-2","username":"other bot"}}`)
	c.handleDispatch(t.Context(), ready, botProfile{})
	if got := c.Status(); got.BotID != "bot-2" || got.BotAvatarURL != "" {
		t.Fatalf("new identity retained previous avatar: %+v", got)
	}
}

type profileTestConn struct{ reads int }

func (c *profileTestConn) Read(context.Context) (websocket.MessageType, []byte, error) {
	c.reads++
	switch c.reads {
	case 1:
		return websocket.MessageText, []byte(`{"op":10,"d":{"heartbeat_interval":30000}}`), nil
	case 2:
		return websocket.MessageText, []byte(`{"op":0,"t":"READY","d":{"session_id":"s1","user":{"id":"bot-1","username":"gateway bot"}}}`), nil
	default:
		return 0, nil, io.EOF
	}
}
func (*profileTestConn) Write(context.Context, websocket.MessageType, []byte) error { return nil }
func (*profileTestConn) Close(websocket.StatusCode, string) error                   { return nil }

func TestConnectionEnrichesReadyWithoutRequiringProfileSuccess(t *testing.T) {
	t.Parallel()
	for _, profileStatus := range []int{http.StatusOK, http.StatusForbidden} {
		t.Run(http.StatusText(profileStatus), func(t *testing.T) {
			c := New("qq", config.QQOfficialConfig{AppID: "10001", AppSecret: "fixture-secret"}, config.AdapterConfig{}, discardLogger())
			c.tokens.token, c.tokens.expiresAt = "fixture-token", time.Now().Add(time.Hour)
			profileRequests := 0
			c.http.Transport = reloadTestTransport(func(r *http.Request) (*http.Response, error) {
				status, body := http.StatusOK, `{"url":"wss://example.com/gateway"}`
				if r.URL.Path == "/users/@me" {
					profileRequests++
					status, body = profileStatus, `{"id":"bot-1","username":"profile bot","avatar":"https://example.com/bot.png"}`
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			c.dialer = func(context.Context, string) (wsConn, error) { return &profileTestConn{}, nil }
			if err := c.runConnection(context.Background()); err != io.EOF {
				t.Fatalf("connection = %v, want fixture EOF after READY", err)
			}
			got := c.Status()
			if profileRequests != 1 || got.State != StateConnected || got.BotID != "bot-1" {
				t.Fatalf("connection did not reach READY: %+v, profile requests=%d", got, profileRequests)
			}
			if profileStatus == http.StatusOK {
				if got.BotName != "profile bot" || got.BotAvatarURL != "https://example.com/bot.png" {
					t.Fatalf("connection lost profile: %+v", got)
				}
			} else if got.BotName != "gateway bot" || got.BotAvatarURL != "" {
				t.Fatalf("failed profile replaced gateway identity: %+v", got)
			}
		})
	}
}
