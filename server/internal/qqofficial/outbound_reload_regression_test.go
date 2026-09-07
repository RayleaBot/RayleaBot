package qqofficial

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

type reloadTestTransport func(*http.Request) (*http.Response, error)

func (f reloadTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSendKeepsOneCredentialSnapshotDuringReload(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		segments []chatevent.MessageSegment
	}{
		{name: "text", segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "fixture"}}}},
		{name: "media", segments: []chatevent.MessageSegment{{Type: "image", Data: map[string]any{"url": "https://media.example.invalid/image.png"}}}},
		{name: "text and media", segments: []chatevent.MessageSegment{{Type: "text", Data: map[string]any{"text": "fixture"}}, {Type: "image", Data: map[string]any{"url": "https://media.example.invalid/image.png"}}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			c := New("qq", config.QQOfficialConfig{AppID: "1001", AppSecret: "old-fixture"}, config.AdapterConfig{ConnectTimeoutSeconds: 2}, discardLogger())
			tokenStarted := make(chan struct{})
			releaseToken := make(chan struct{})
			var releaseOnce, tokenOnce sync.Once
			defer releaseOnce.Do(func() { close(releaseToken) })
			requestSent := make(chan *http.Request, 8)
			c.http.Transport = reloadTestTransport(func(r *http.Request) (*http.Response, error) {
				body := `{"id":"sent"}`
				if strings.Contains(r.URL.Path, "getAppAccessToken") {
					tokenOnce.Do(func() { close(tokenStarted) })
					<-releaseToken
					body = `{"access_token":"old-fixture-token","expires_in":7200}`
				} else {
					requestSent <- r
					if strings.HasSuffix(r.URL.Path, "/files") {
						body = `{"file_info":"fixture-file"}`
					}
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})
			finished := make(chan error, 1)
			go func() {
				_, err := c.SendMessage(context.Background(), chatevent.OutboundMessageSend{TargetType: "group", TargetID: "group-fixture", Segments: testCase.segments})
				finished <- err
			}()
			select {
			case <-tokenStarted:
			case <-time.After(time.Second):
				t.Fatal("token exchange did not start")
			}
			c.Reload(config.QQOfficialConfig{AppID: "2002", AppSecret: "new-fixture", Sandbox: true})
			releaseOnce.Do(func() { close(releaseToken) })
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("send did not complete")
			}
			close(requestSent)
			count := 0
			for r := range requestSent {
				count++
				if r.Header.Get("Authorization") != "QQBot old-fixture-token" || r.Header.Get("X-Union-Appid") != "1001" || r.URL.Host != "api.sgroup.qq.com" {
					t.Fatalf("mixed snapshots: endpoint %s sent with app_id=%s", r.URL.Host, r.Header.Get("X-Union-Appid"))
				}
			}
			if count == 0 {
				t.Fatal("no message or media request sent")
			}
		})
	}
}

func TestEnableSwitchCancelsConnectionAndCanStartAgain(t *testing.T) {
	c := New("qq", config.QQOfficialConfig{AppID: "1001", AppSecret: "fixture"}, config.AdapterConfig{ConnectTimeoutSeconds: 5, ReconnectInitialSeconds: 30}, discardLogger())
	requests := make(chan context.Context, 2)
	c.http.Transport = reloadTestTransport(func(r *http.Request) (*http.Response, error) {
		requests <- r.Context()
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	c.SetEnabled(false)
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		if err := c.Stop(stopCtx); err != nil {
			t.Error(err)
		}
	}()
	c.Start(ctx)
	select {
	case <-requests:
		t.Fatal("disabled client attempted a connection")
	case <-time.After(30 * time.Millisecond):
	}
	for range 2 {
		c.SetEnabled(true)
		var requestCtx context.Context
		select {
		case requestCtx = <-requests:
		case <-time.After(time.Second):
			t.Fatal("enabled client did not connect")
		}
		c.SetEnabled(false)
		select {
		case <-requestCtx.Done():
		case <-time.After(time.Second):
			t.Fatal("disabling did not cancel token exchange")
		}
		if got := c.Status().State; got != StateStopped {
			t.Fatalf("disabled state=%s", got)
		}
	}
}

func TestResumedRestoresConnectedStatus(t *testing.T) {
	c := &Client{logger: discardLogger()}
	c.session.startSession("session", "bot", "fixture")
	c.setState(StateConnecting, "")
	c.handleDispatch(context.Background(), gatewayFrame{Op: opDispatch, T: dispatchResumed})
	if got := c.Status().State; got != StateConnected {
		t.Fatalf("after RESUMED: state=%s, want connected", got)
	}
}
