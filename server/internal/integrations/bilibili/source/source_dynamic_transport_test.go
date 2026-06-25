package source

import (
	"bytes"
	"context"
	bilibilimonitoring "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/monitoring"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func monitorDynamicTransport(t *testing.T, now time.Time, itemJSON string) func(*http.Request) (*http.Response, error) {
	t.Helper()
	return func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "api.bilibili.com/bapis/bilibili.api.ticket.v1.Ticket/GenWebTicket":
			return jsonResponse(`{
				"code": 0,
				"data": {
					"ticket": "ticket-value",
					"created_at": ` + strconv.FormatInt(now.Unix(), 10) + `,
					"ttl": 259200,
					"nav": {
						"img": "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png",
						"sub": "https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png"
					}
				}
			}`), nil
		case "api.bilibili.com/x/polymer/web-dynamic/v1/feed/space":
			if request.URL.Query().Get("host_mid") != "123456" {
				t.Fatalf("unexpected space feed host_mid: %s", request.URL.String())
			}
			if request.URL.Query().Get("wts") != strconv.FormatInt(now.Unix(), 10) || request.URL.Query().Get("w_rid") == "" {
				t.Fatalf("space feed url missing WBI signature: %s", request.URL.String())
			}
			if request.Header.Get("Cookie") != monitorCookie {
				t.Fatalf("unexpected space feed cookie: %q", request.Header.Get("Cookie"))
			}
			items := ""
			if strings.TrimSpace(itemJSON) != "" {
				items = itemJSON
			}
			return jsonResponse(`{"code":0,"data":{"items":[` + items + `]}}`), nil
		default:
			t.Fatalf("unexpected request url: %s", request.URL.String())
			return nil, nil
		}
	}
}

func seedBilibiliAccount(t *testing.T, source *Source, ctx context.Context) {
	t.Helper()
	_, err := source.accounts.Upsert(ctx, thirdparty.UpsertRequest{
		Platform:  thirdparty.PlatformBilibili,
		AccountID: "primary",
		Label:     "主账号",
		Enabled:   true,
		Cookie:    monitorCookie,
		Profile: thirdparty.AccountProfile{
			UID:       "primary",
			Nickname:  "主账号",
			AvatarURL: "https://i0.hdslb.com/bfs/face/account.jpg",
		},
		Credential: thirdparty.CredentialStatus{
			State: thirdparty.CredentialValid,
		},
	})
	if err != nil {
		t.Fatalf("seed bilibili account: %v", err)
	}
}

type dispatchRecorder struct {
	events []recordedBilibiliEvent
}

type recordedBilibiliEvent struct {
	BilibiliEvent
	Timestamp int64
}

func (r *dispatchRecorder) DispatchBilibiliEvent(_ context.Context, event BilibiliEvent, timestamp int64) {
	r.events = append(r.events, recordedBilibiliEvent{BilibiliEvent: event, Timestamp: timestamp})
}

type staticPluginConfig struct {
	values map[string]any
}

func (staticPluginConfig) SeedDefaults(context.Context, string, map[string]any) (bool, error) {
	return false, nil
}

func (staticPluginConfig) Read(context.Context, string, []string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (s staticPluginConfig) ReadAll(context.Context, string) (map[string]any, error) {
	if s.values == nil {
		return map[string]any{}, nil
	}
	return s.values, nil
}

func (staticPluginConfig) Write(context.Context, string, map[string]any) ([]string, error) {
	return nil, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	if fn == nil {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewBufferString(`{"code":0,"data":{}}`)),
			Request:    request,
		}, nil
	}
	return fn(request)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func bilibiliPayload(t *testing.T, event recordedBilibiliEvent) map[string]any {
	t.Helper()
	payload := bilibilimonitoring.Payload(event.BilibiliEvent)
	if payload == nil {
		t.Fatalf("event missing bilibili payload: %#v", event)
	}
	return payload
}

func containsText(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
