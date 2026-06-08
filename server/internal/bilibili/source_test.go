package bilibili

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func TestLiveTransitionDispatchesStartedEndedAndDeduplicates(t *testing.T) {
	t.Parallel()

	source, recorder := newTestSource(t, time.Date(2026, 6, 8, 8, 10, 0, 0, time.UTC), nil)
	ctx := context.Background()
	subject := Subject{
		UID:       "123456",
		Name:      "测试主播",
		AvatarURL: "https://i0.hdslb.com/bfs/face/default.jpg",
		Services:  map[string]bool{"live": true},
	}
	item := liveStatusItem{
		UID:           "123456",
		UName:         "测试主播",
		Face:          "//i0.hdslb.com/bfs/face/live.jpg",
		RoomID:        "10001",
		Title:         "直播间标题",
		LiveStatus:    1,
		LiveTime:      int64(1780906000),
		URL:           "https://live.bilibili.com/10001",
		CoverFromUser: "//i0.hdslb.com/bfs/live/cover.jpg",
	}

	source.emitLiveTransition(ctx, subject, item, 1, "websocket")
	source.emitLiveTransition(ctx, subject, item, 1, "websocket")
	if len(recorder.events) != 1 {
		t.Fatalf("live started events = %d, want 1", len(recorder.events))
	}
	started := recorder.events[0]
	if started.EventType != EventLiveStarted || started.SourceProtocol != sourceProtocol || started.SourceAdapter != sourceAdapter {
		t.Fatalf("unexpected live started event: %#v", started)
	}
	startedPayload := bilibiliPayload(t, started)
	if startedPayload["kind"] != "live" || startedPayload["uid"] != "123456" || startedPayload["room_id"] != "10001" || startedPayload["service"] != "live" {
		t.Fatalf("unexpected live started payload: %#v", startedPayload)
	}
	if startedPayload["live_event"] != "started" || startedPayload["status_label"] != "直播中" || startedPayload["live_status"] != 1 {
		t.Fatalf("unexpected live started status payload: %#v", startedPayload)
	}
	if images, ok := startedPayload["images"].([]map[string]any); !ok || len(images) != 1 || images[0]["url"] != "https://i0.hdslb.com/bfs/live/cover.jpg" {
		t.Fatalf("unexpected live started images: %#v", startedPayload["images"])
	}

	endedItem := item
	endedItem.LiveStatus = 0
	source.emitLiveTransition(ctx, subject, endedItem, 0, "websocket")
	source.emitLiveTransition(ctx, subject, endedItem, 0, "websocket")
	if len(recorder.events) != 2 {
		t.Fatalf("live events = %d, want 2", len(recorder.events))
	}
	ended := recorder.events[1]
	if ended.EventType != EventLiveEnded {
		t.Fatalf("unexpected live ended event type: %#v", ended.EventType)
	}
	endedPayload := bilibiliPayload(t, ended)
	if endedPayload["live_event"] != "ended" || endedPayload["status_label"] != "直播结束" || endedPayload["live_status"] != 0 {
		t.Fatalf("unexpected live ended payload: %#v", endedPayload)
	}
}

func TestPollDynamicsDispatchesWatchedUpdatesAndDeduplicates(t *testing.T) {
	t.Parallel()

	source, recorder := newTestSource(t, time.Date(2026, 6, 8, 8, 11, 0, 0, time.UTC), func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != dynamicFeedURL {
			t.Fatalf("unexpected dynamic feed url: %s", request.URL.String())
		}
		if request.Header.Get("Cookie") != "SESSDATA=fixture; bili_jct=csrf;" {
			t.Fatalf("unexpected dynamic feed cookie: %q", request.Header.Get("Cookie"))
		}
		return jsonResponse(`{
			"code": 0,
			"data": {
				"items": [{
					"id_str": "90001",
					"type": "DYNAMIC_TYPE_AV",
					"basic": {"jump_url": "//www.bilibili.com/video/BV1RayleaBot"},
					"modules": {
						"module_author": {
							"mid": "123456",
							"name": "测试 UP",
							"face": "//i0.hdslb.com/bfs/face/up.jpg",
							"pub_ts": 1780906200
						},
						"module_dynamic": {
							"major": {
								"type": "MAJOR_TYPE_ARCHIVE",
								"archive": {
									"title": "新视频标题",
									"desc": "视频简介",
									"cover": "//i0.hdslb.com/bfs/archive/cover.jpg"
								}
							}
						}
					}
				}, {
					"id_str": "90002",
					"type": "DYNAMIC_TYPE_DRAW",
					"modules": {
						"module_author": {"mid": "999999", "name": "未订阅 UP", "pub_ts": 1780906210},
						"module_dynamic": {"desc": {"text": "未订阅动态"}}
					}
				}]
			}
		}`), nil
	})

	subjects := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "测试 UP",
			Services: map[string]bool{"video": true},
		},
	}
	ctx := context.Background()
	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}
	source.pollDynamics(ctx, subjects, account, "SESSDATA=fixture; bili_jct=csrf;")
	source.pollDynamics(ctx, subjects, account, "SESSDATA=fixture; bili_jct=csrf;")

	if len(recorder.events) != 1 {
		t.Fatalf("dynamic events = %d, want 1", len(recorder.events))
	}
	event := recorder.events[0]
	if event.EventType != EventDynamicPublished || event.Timestamp != 1780906200 {
		t.Fatalf("unexpected dynamic event: %#v", event)
	}
	payload := bilibiliPayload(t, event)
	if payload["kind"] != "dynamic" || payload["uid"] != "123456" || payload["id"] != "90001" || payload["service"] != "video" {
		t.Fatalf("unexpected dynamic payload identity: %#v", payload)
	}
	if payload["title"] != "新视频标题" || payload["summary"] != "视频简介" || payload["url"] != "https://www.bilibili.com/video/BV1RayleaBot" {
		t.Fatalf("unexpected dynamic content payload: %#v", payload)
	}
	author, ok := payload["author"].(map[string]any)
	if !ok || author["uid"] != "123456" || author["name"] != "测试 UP" {
		t.Fatalf("unexpected dynamic author payload: %#v", payload["author"])
	}
}

func TestEnsureRoomTasksRestartsWhenCookieChanges(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 12, 0, 0, time.UTC), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subjects := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "测试主播",
			Services: map[string]bool{"live": true},
		},
	}

	source.ensureRoomTasks(ctx, subjects, "SESSDATA=old;")
	oldTask := source.roomTasks["123456"]
	source.ensureRoomTasks(ctx, subjects, "SESSDATA=old;")
	if source.roomTasks["123456"].cookieFingerprint != oldTask.cookieFingerprint {
		t.Fatalf("expected unchanged cookie to keep task")
	}
	source.ensureRoomTasks(ctx, subjects, "SESSDATA=new;")
	newTask := source.roomTasks["123456"]
	if newTask.cookieFingerprint == oldTask.cookieFingerprint {
		t.Fatalf("expected changed cookie to restart task")
	}
	select {
	case <-oldTask.ctx.Done():
	default:
		t.Fatalf("expected old live room task to be cancelled")
	}
}

func TestUpdateWatchCountsIgnoresUnwatchedRoomState(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 13, 0, 0, time.UTC), nil)
	ctx := context.Background()
	source.setRoomState(ctx, roomState{
		UID:             "old",
		ConnectionState: StateFailed,
		UpdatedAt:       time.Date(2026, 6, 8, 8, 12, 0, 0, time.UTC),
	})
	source.setRoomState(ctx, roomState{
		UID:             "123456",
		ConnectionState: StateConnected,
		UpdatedAt:       time.Date(2026, 6, 8, 8, 12, 0, 0, time.UTC),
	})

	source.updateWatchCounts(ctx, map[string]Subject{
		"123456": {
			UID:      "123456",
			Services: map[string]bool{"live": true},
		},
	})

	status := source.Status(ctx)
	if status.Live.WatchedRooms != 1 || status.Live.ConnectedRooms != 1 || status.Live.FailedRooms != 0 || status.Status != StateConnected {
		t.Fatalf("unexpected watched status: %#v", status)
	}
}

func TestPrimaryAccountCookieSkipsInvalidCredentials(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 14, 0, 0, time.UTC), nil)
	ctx := context.Background()
	invalidCheckedAt := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	if _, err := source.accounts.Upsert(ctx, thirdparty.UpsertRequest{
		Platform:  thirdparty.PlatformBilibili,
		AccountID: "invalid",
		Label:     "无效 CK",
		Enabled:   true,
		Cookie:    "SESSDATA=invalid;",
		Credential: thirdparty.CredentialStatus{
			State:     thirdparty.CredentialInvalid,
			CheckedAt: &invalidCheckedAt,
			LastError: "账号未登录",
		},
	}); err != nil {
		t.Fatalf("upsert invalid account: %v", err)
	}
	validCheckedAt := time.Date(2026, 6, 8, 8, 1, 0, 0, time.UTC)
	if _, err := source.accounts.Upsert(ctx, thirdparty.UpsertRequest{
		Platform:  thirdparty.PlatformBilibili,
		AccountID: "valid",
		Label:     "有效 CK",
		Enabled:   true,
		Cookie:    "SESSDATA=valid;",
		Credential: thirdparty.CredentialStatus{
			State:     thirdparty.CredentialValid,
			CheckedAt: &validCheckedAt,
		},
	}); err != nil {
		t.Fatalf("upsert valid account: %v", err)
	}

	account, cookie, err := source.primaryAccountCookie(ctx)
	if err != nil {
		t.Fatalf("primaryAccountCookie: %v", err)
	}
	if account.AccountID != "valid" || cookie != "SESSDATA=valid;" {
		t.Fatalf("unexpected primary account %q cookie %q", account.AccountID, cookie)
	}
}

func newTestSource(t *testing.T, now time.Time, handler func(*http.Request) (*http.Response, error)) (*Source, *dispatchRecorder) {
	t.Helper()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	secretStore, err := secrets.NewSQLiteStore(store)
	if err != nil {
		t.Fatalf("secrets.NewSQLiteStore: %v", err)
	}
	accounts, err := thirdparty.NewService(store, secretStore)
	if err != nil {
		t.Fatalf("thirdparty.NewService: %v", err)
	}
	recorder := &dispatchRecorder{}
	source, err := NewSource(Deps{
		Store:         store,
		Accounts:      accounts,
		PluginConfig:  staticPluginConfig{},
		Dispatcher:    recorder,
		HTTPTransport: roundTripFunc(handler),
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	return source, recorder
}

type dispatchRecorder struct {
	events []runtime.Event
}

func (r *dispatchRecorder) Dispatch(_ context.Context, event runtime.Event, _ string) []dispatch.DeliveryResult {
	r.events = append(r.events, event)
	return []dispatch.DeliveryResult{{PluginID: subscriptionHubPluginID, Outcome: dispatch.OutcomeDelivered}}
}

type staticPluginConfig struct{}

func (staticPluginConfig) SeedDefaults(context.Context, string, map[string]any) (bool, error) {
	return false, nil
}

func (staticPluginConfig) Read(context.Context, string, []string) (map[string]any, error) {
	return map[string]any{}, nil
}

func (staticPluginConfig) ReadAll(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
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

func bilibiliPayload(t *testing.T, event runtime.Event) map[string]any {
	t.Helper()
	payload, ok := event.PayloadFields["bilibili"].(map[string]any)
	if !ok {
		t.Fatalf("event missing bilibili payload: %#v", event.PayloadFields)
	}
	return payload
}
