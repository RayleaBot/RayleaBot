package source

import (
	"context"
	bilibiliDynamic "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/dynamic"
	bilibilisubscriptions "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/subscriptions"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/RayleaBot/RayleaBot/server/internal/secrets"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMonitorSnapshotKeepsPinnedDynamicWhenItIsLatest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 28, 0, 0, time.UTC)
	source, _ := newTestSourceWithPluginConfig(t, now, monitorDynamicTransport(t, now, `{
		"id_str": "100000000000000008",
		"type": "DYNAMIC_TYPE_AV",
		"modules": {
			"module_tag": {"text": "置顶"},
			"module_author": {"mid": "123456", "name": "测试 UP", "pub_ts": 1780906300},
			"module_dynamic": {
				"major": {
					"type": "MAJOR_TYPE_ARCHIVE",
					"archive": {"title": "置顶也是最新"}
				}
			}
		}
	}, {
		"id_str": "100000000000000004",
		"type": "DYNAMIC_TYPE_DRAW",
		"modules": {
			"module_author": {"mid": "123456", "name": "测试 UP", "pub_ts": 1780906200},
			"module_dynamic": {
				"desc": {"text": "较早图文"},
				"major": {
					"type": "MAJOR_TYPE_DRAW",
					"draw": {"items": [{"src": "//i0.hdslb.com/bfs/album/previous.jpg"}]}
				}
			}
		}
	}`), staticPluginConfig{
		values: map[string]any{
			"subscriptions": []any{
				map[string]any{
					"enabled":  true,
					"platform": "bilibili",
					"uid":      "123456",
					"name":     "测试 UP",
					"services": []any{"video"},
				},
			},
		},
	})
	ctx := context.Background()
	seedBilibiliAccount(t, source, ctx)

	snapshot, err := source.MonitorSnapshot(ctx)
	if err != nil {
		t.Fatalf("MonitorSnapshot: %v", err)
	}
	if len(snapshot.Items) != 1 || snapshot.Items[0].Dynamic == nil {
		t.Fatalf("unexpected monitor snapshot: %#v", snapshot)
	}
	dynamic := snapshot.Items[0].Dynamic
	if dynamic.LastID != "100000000000000008" || dynamic.Title != "置顶也是最新" {
		t.Fatalf("unexpected pinned latest dynamic: %#v", dynamic)
	}
}

func TestMonitorSnapshotClearsDynamicWhenSpaceFeedHasNoDisplayableItem(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 25, 0, 0, time.UTC)
	source, _ := newTestSourceWithPluginConfig(t, now, monitorDynamicTransport(t, now, ""), staticPluginConfig{
		values: map[string]any{
			"subscriptions": []any{
				map[string]any{
					"enabled":  true,
					"platform": "bilibili",
					"uid":      "123456",
					"name":     "测试 UP",
					"services": []any{"video"},
				},
			},
		},
	})
	ctx := context.Background()
	seedBilibiliAccount(t, source, ctx)
	source.stateStore.SetDynamic(ctx, BilibiliEvent{
		UID:     "123456",
		ID:      "old-90001",
		Service: "video",
		Title:   "旧动态",
		URL:     "https://t.bilibili.com/old-90001/",
		Author:  Author{Name: "测试 UP"},
	})

	snapshot, err := source.MonitorSnapshot(ctx)
	if err != nil {
		t.Fatalf("MonitorSnapshot: %v", err)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("monitor items = %d, want 1", len(snapshot.Items))
	}
	if snapshot.Items[0].Dynamic != nil {
		t.Fatalf("expected dynamic snapshot to be cleared, got %#v", snapshot.Items[0].Dynamic)
	}
}

func TestMonitorSnapshotDoesNotShowDynamicForLiveOnlySubject(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 26, 0, 0, time.UTC)
	source, _ := newTestSourceWithPluginConfig(t, now, nil, staticPluginConfig{
		values: map[string]any{
			"subscriptions": []any{
				map[string]any{
					"enabled":  true,
					"platform": "bilibili",
					"uid":      "123456",
					"name":     "测试 UP",
					"services": []any{"live"},
				},
			},
		},
	})
	ctx := context.Background()
	source.stateStore.SetDynamic(ctx, BilibiliEvent{
		UID:     "123456",
		ID:      "old-90001",
		Service: "video",
		Title:   "旧动态",
		URL:     "https://t.bilibili.com/old-90001/",
		Author:  Author{Name: "测试 UP"},
	})

	snapshot, err := source.MonitorSnapshot(ctx)
	if err != nil {
		t.Fatalf("MonitorSnapshot: %v", err)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("monitor items = %d, want 1", len(snapshot.Items))
	}
	if snapshot.Items[0].Dynamic != nil {
		t.Fatalf("live-only subject dynamic = %#v, want nil", snapshot.Items[0].Dynamic)
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
	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}

	source.ensureRoomTasks(ctx, subjects, account, "SESSDATA=old;")
	oldTask := source.roomTasks["123456"]
	source.ensureRoomTasks(ctx, subjects, account, "SESSDATA=old;")
	if source.roomTasks["123456"].cookieFingerprint != oldTask.cookieFingerprint {
		t.Fatalf("expected unchanged cookie to keep task")
	}
	source.ensureRoomTasks(ctx, subjects, account, "SESSDATA=new;")
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

func TestEnsureRoomTasksStopsWhenCookieMissing(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 12, 30, 0, time.UTC), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	subjects := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "测试主播",
			Services: map[string]bool{"live": true},
		},
	}
	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}

	source.ensureRoomTasks(ctx, subjects, account, "SESSDATA=fixture;")
	task := source.roomTasks["123456"]
	source.ensureRoomTasks(ctx, subjects, thirdparty.Account{}, "")
	if len(source.roomTasks) != 0 {
		t.Fatalf("room tasks = %d, want 0", len(source.roomTasks))
	}
	select {
	case <-task.ctx.Done():
	default:
		t.Fatalf("expected live room task to be cancelled")
	}
}

func TestMonitorSnapshotSuppressesStoredRiskControlRoomErrors(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 18, 0, 0, time.UTC)
	source, _ := newTestSourceWithPluginConfig(t, now, nil, staticPluginConfig{
		values: map[string]any{
			"subscriptions": []any{
				map[string]any{
					"enabled":  true,
					"platform": "bilibili",
					"uid":      "123456",
					"name":     "测试 UP",
					"services": []any{"live"},
				},
			},
		},
	})
	ctx := context.Background()
	source.stateStore.SetRoom(ctx, sourceRoom{
		UID:             "123456",
		Name:            "测试 UP",
		Face:            "https://i0.hdslb.com/bfs/face/live.jpg",
		ConnectionState: StateDegraded,
		LastError:       `bilibili: risk_control: code -352: HTTP 200: {"code":-352,"message":"-352","ttl":1}`,
		UpdatedAt:       now,
	})

	snapshot, err := source.MonitorSnapshot(ctx)
	if err != nil {
		t.Fatalf("MonitorSnapshot: %v", err)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("monitor items = %d, want 1", len(snapshot.Items))
	}
	item := snapshot.Items[0]
	if item.Live.LastError != "" || item.Live.ConnectionState != StateIdle {
		t.Fatalf("unexpected monitor live state: %#v", item.Live)
	}
	if item.AvatarURL != "https://i0.hdslb.com/bfs/face/live.jpg" {
		t.Fatalf("avatar url = %q, want room face", item.AvatarURL)
	}
}

func TestMonitorSnapshotDoesNotGuessLiveEndedAtFromRoomUpdate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 18, 15, 0, time.UTC)
	source, _ := newTestSourceWithPluginConfig(t, now, nil, staticPluginConfig{
		values: map[string]any{
			"subscriptions": []any{
				map[string]any{
					"enabled":  true,
					"platform": "bilibili",
					"uid":      "123456",
					"name":     "测试 UP",
					"services": []any{"live"},
				},
			},
		},
	})
	ctx := context.Background()
	source.stateStore.SetRoom(ctx, sourceRoom{
		UID:             "123456",
		Name:            "测试 UP",
		LiveStatus:      0,
		ConnectionState: StateIdle,
		LastEventAt:     &now,
		UpdatedAt:       now,
	})

	snapshot, err := source.MonitorSnapshot(ctx)
	if err != nil {
		t.Fatalf("MonitorSnapshot: %v", err)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("monitor items = %d, want 1", len(snapshot.Items))
	}
	if snapshot.Items[0].Live.LiveEndedAt != nil {
		t.Fatalf("live_ended_at = %#v, want nil", snapshot.Items[0].Live.LiveEndedAt)
	}
}

func TestPollDynamicsRiskControlCoolsDynamicWithoutInvalidatingAccount(t *testing.T) {
	t.Parallel()

	requests := 0
	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 18, 30, 0, time.UTC), func(request *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(`{"code":-352,"message":"-352","ttl":1}`), nil
	})
	ctx := context.Background()
	checkedAt := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	account, err := source.accounts.Upsert(ctx, thirdparty.UpsertRequest{
		Platform:  thirdparty.PlatformBilibili,
		AccountID: "primary",
		Label:     "主账号",
		Enabled:   true,
		Cookie:    "SESSDATA=fixture;",
		Profile: thirdparty.AccountProfile{
			UID:       "primary",
			Nickname:  "主账号",
			AvatarURL: "https://i0.hdslb.com/bfs/face/account.jpg",
		},
		Credential: thirdparty.CredentialStatus{
			State:     thirdparty.CredentialValid,
			CheckedAt: &checkedAt,
		},
	})
	if err != nil {
		t.Fatalf("upsert account: %v", err)
	}

	source.pollDynamics(ctx, map[string]Subject{
		"123456": {
			UID:      "123456",
			Services: map[string]bool{"video": true},
		},
	}, account, "SESSDATA=fixture;")

	accounts, err := source.accounts.List(ctx)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 || accounts[0].Credential.State != thirdparty.CredentialValid || accounts[0].Credential.LastError != "" {
		t.Fatalf("unexpected account credential: %#v", accounts)
	}
	status := source.Status(ctx)
	if !strings.Contains(status.Dynamic.LastError, "code -352") {
		t.Fatalf("unexpected dynamic error: %#v", status.Dynamic.LastError)
	}
	if status.Diagnosis.Level != "attention" || status.Diagnosis.Headline != "平台风控等待中" {
		t.Fatalf("unexpected dynamic risk diagnosis: %#v", status.Diagnosis)
	}
	if len(status.Diagnosis.Causes) != 1 || status.Diagnosis.Causes[0].Scope != "dynamic" || status.Diagnosis.Causes[0].Code != "platform_risk_control" || status.Diagnosis.Causes[0].RetryAt == nil {
		t.Fatalf("unexpected dynamic risk cause: %#v", status.Diagnosis.Causes)
	}
	if !containsText(status.Diagnosis.Impacts, "动态检查暂时等待平台恢复。") || !containsText(status.Diagnosis.Impacts, "CK 有效，无需重新登录。") {
		t.Fatalf("unexpected dynamic risk impacts: %#v", status.Diagnosis.Impacts)
	}
	requestsAfterRisk := requests
	source.pollDynamics(ctx, map[string]Subject{
		"123456": {
			UID:      "123456",
			Services: map[string]bool{"video": true},
		},
	}, account, "SESSDATA=fixture;")
	if requests != requestsAfterRisk {
		t.Fatalf("dynamic cooldown should skip immediate retry, requests = %d before = %d", requests, requestsAfterRisk)
	}
}

func TestLiveRiskControlDoesNotBlockDynamicCookieUse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 23, 0, 0, time.UTC)
	source, recorder := newTestSourceWithPluginConfig(t, now, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "api.live.bilibili.com/room/v1/Room/get_status_info_by_uids":
			return jsonResponse(`{"code":-352,"message":"-352","ttl":1}`), nil
		case "api.bilibili.com/bapis/bilibili.api.ticket.v1.Ticket/GenWebTicket":
			return jsonResponse(`{
				"code": 0,
				"data": {
					"ticket": "ticket-value",
					"created_at": 1780906980,
					"ttl": 259200,
					"nav": {
						"img": "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png",
						"sub": "https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png"
					}
				}
			}`), nil
		case "api.bilibili.com/x/polymer/web-dynamic/v1/feed/all":
			return jsonResponse(`{"code":0,"data":{"items":[{
				"id_str": "dynamic-after-live-risk",
				"type": "DYNAMIC_TYPE_AV",
				"basic": {"jump_url": "//www.bilibili.com/video/BV1AFTERLIVERISK"},
				"modules": {
					"module_author": {"mid": "123456", "name": "测试 UP", "pub_ts": 1780906980},
					"module_dynamic": {
						"major": {
							"type": "MAJOR_TYPE_ARCHIVE",
							"archive": {"title": "直播风控后的动态", "desc": "动态仍可用", "cover": "//i0.hdslb.com/bfs/archive/after-live-risk.jpg"}
						}
					}
				}
			}]}}`), nil
		default:
			t.Fatalf("unexpected request url: %s", request.URL.String())
			return nil, nil
		}
	}, staticPluginConfig{
		values: map[string]any{
			"subscriptions": []any{
				map[string]any{
					"enabled":  true,
					"platform": "bilibili",
					"uid":      "123456",
					"name":     "测试 UP",
					"services": []any{"live", "video"},
				},
			},
		},
	})
	ctx := context.Background()
	checkedAt := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	account, err := source.accounts.Upsert(ctx, thirdparty.UpsertRequest{
		Platform:  thirdparty.PlatformBilibili,
		AccountID: "primary",
		Label:     "主账号",
		Enabled:   true,
		Cookie:    "SESSDATA=fixture; bili_jct=csrf;",
		Credential: thirdparty.CredentialStatus{
			State:     thirdparty.CredentialValid,
			CheckedAt: &checkedAt,
		},
	})
	if err != nil {
		t.Fatalf("upsert account: %v", err)
	}
	subjects := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "测试 UP",
			Services: map[string]bool{"live": true, "video": true},
		},
	}

	source.updateWatchCounts(ctx, subjects)
	source.pollLiveFallback(ctx, subjects, account, "SESSDATA=fixture; bili_jct=csrf;")
	accounts, err := source.accounts.List(ctx)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(accounts) != 1 || accounts[0].Credential.State != thirdparty.CredentialValid {
		t.Fatalf("live risk should not invalidate account: %#v", accounts)
	}
	source.markSeen(ctx, EventDynamicPublished+":baseline", "123456", EventDynamicPublished, "baseline")
	source.pollDynamics(ctx, subjects, account, "SESSDATA=fixture; bili_jct=csrf;")

	if len(recorder.events) != 1 {
		t.Fatalf("dynamic events after live risk = %d, want 1", len(recorder.events))
	}
	payload := bilibiliPayload(t, recorder.events[0])
	if payload["id"] != "dynamic-after-live-risk" || payload["title"] != "直播风控后的动态" {
		t.Fatalf("unexpected dynamic payload after live risk: %#v", payload)
	}
	status := source.Status(ctx)
	if !strings.Contains(status.Live.LastError, "code -352") {
		t.Fatalf("expected live risk error to remain isolated, status = %#v", status)
	}
	if status.Dynamic.LastError != "" {
		t.Fatalf("dynamic status should remain healthy, got %q", status.Dynamic.LastError)
	}
	if status.Diagnosis.Headline != "平台风控等待中" {
		t.Fatalf("unexpected live risk diagnosis: %#v", status.Diagnosis)
	}
	if !containsText(status.Diagnosis.Impacts, "动态接收不受影响。") || !containsText(status.Diagnosis.Impacts, "CK 有效，无需重新登录。") {
		t.Fatalf("live risk should explain dynamic and CK impact: %#v", status.Diagnosis.Impacts)
	}
}

func TestPollLiveFallbackClearsTransientLiveCheckError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 23, 30, 0, time.UTC)
	liveRequests := 0
	source, _ := newTestSourceWithPluginConfig(t, now, func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "api.live.bilibili.com/room/v1/Room/get_status_info_by_uids":
			liveRequests++
			if liveRequests == 1 {
				return nil, io.ErrUnexpectedEOF
			}
			return jsonResponse(`{"code":0,"data":{"123456":{
				"uid":123456,
				"uname":"测试 UP",
				"face":"//i0.hdslb.com/bfs/face/up.jpg",
				"room_id":10001,
				"title":"直播间标题",
				"live_status":0,
				"url":"https://live.bilibili.com/10001"
			}}}`), nil
		default:
			t.Fatalf("unexpected request url: %s", request.URL.String())
			return nil, nil
		}
	}, staticPluginConfig{
		values: map[string]any{
			"subscriptions": []any{
				map[string]any{
					"enabled":  true,
					"platform": "bilibili",
					"uid":      "123456",
					"name":     "测试 UP",
					"services": []any{"live", "video"},
				},
			},
		},
	})
	ctx := context.Background()
	var published []Status
	source.notifyStatus = func(status Status) {
		published = append(published, status)
	}
	subjects := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "测试 UP",
			Services: map[string]bool{"live": true, "video": true},
		},
	}
	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}

	source.updateWatchCounts(ctx, subjects)
	source.mu.Lock()
	source.status.Dynamic.LastPollAt = &now
	source.refreshStatusLocked(nil)
	source.mu.Unlock()

	source.pollLiveFallback(ctx, subjects, account, monitorCookie)
	statusAfterError := source.Status(ctx)
	if statusAfterError.Status != StateConnected || statusAfterError.Diagnosis.Level == "action_required" {
		t.Fatalf("transient live check error should not require action: %#v", statusAfterError)
	}
	if !strings.Contains(statusAfterError.Live.LastError, "unexpected EOF") {
		t.Fatalf("expected live error to be retained for diagnostics, got %#v", statusAfterError.Live.LastError)
	}

	source.pollLiveFallback(ctx, subjects, account, monitorCookie)
	statusAfterRecovery := source.Status(ctx)
	if statusAfterRecovery.Live.LastError != "" {
		t.Fatalf("live error should be cleared after successful fallback check: %#v", statusAfterRecovery.Live.LastError)
	}
	if statusAfterRecovery.Status != StateConnected || statusAfterRecovery.Diagnosis.Level != "normal" {
		t.Fatalf("unexpected recovered status: %#v", statusAfterRecovery)
	}
	if len(published) == 0 || published[len(published)-1].Live.LastError != "" {
		t.Fatalf("expected recovered status publication, got %#v", published)
	}
}

func TestSourceDiagnosisExplainsLiveFallback(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 24, 0, 0, time.UTC), nil)
	ctx := context.Background()
	source.stateStore.SetRoom(ctx, sourceRoom{
		UID:             "123456",
		ConnectionState: StateDegraded,
		LastError:       "直播间 123456 连接失败",
		UpdatedAt:       time.Date(2026, 6, 8, 8, 23, 0, 0, time.UTC),
	})
	source.updateWatchCounts(ctx, map[string]Subject{
		"123456": {
			UID:      "123456",
			Services: map[string]bool{"live": true, "video": true},
		},
	})

	status := source.Status(ctx)
	if status.Summary != "Bilibili 事件源运行受限" {
		t.Fatalf("summary = %q", status.Summary)
	}
	if status.Diagnosis.Level != "attention" || status.Diagnosis.Headline != "直播备用检查中" {
		t.Fatalf("unexpected live fallback diagnosis: %#v", status.Diagnosis)
	}
	if len(status.Diagnosis.Causes) != 1 || status.Diagnosis.Causes[0].Code != "live_fallback" {
		t.Fatalf("unexpected live fallback cause: %#v", status.Diagnosis.Causes)
	}
	if !containsText(status.Diagnosis.Impacts, "动态接收不受影响。") {
		t.Fatalf("live fallback should explain dynamic impact: %#v", status.Diagnosis.Impacts)
	}
}

func TestSourceDiagnosisPrioritizesInvalidCredential(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 25, 0, 0, time.UTC), nil)
	ctx := context.Background()
	checkedAt := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	if _, err := source.accounts.Upsert(ctx, thirdparty.UpsertRequest{
		Platform:  thirdparty.PlatformBilibili,
		AccountID: "primary",
		Label:     "主账号",
		Enabled:   true,
		Cookie:    "SESSDATA=invalid;",
		Credential: thirdparty.CredentialStatus{
			State:     thirdparty.CredentialInvalid,
			CheckedAt: &checkedAt,
			LastError: "账号未登录",
		},
	}); err != nil {
		t.Fatalf("upsert account: %v", err)
	}
	source.updateWatchCounts(ctx, map[string]Subject{
		"123456": {
			UID:      "123456",
			Services: map[string]bool{"live": true, "video": true},
		},
	})

	status := source.Status(ctx)
	if status.Diagnosis.Level != "action_required" || status.Diagnosis.Headline != "CK 需要重新登录" {
		t.Fatalf("unexpected invalid credential diagnosis: %#v", status.Diagnosis)
	}
	if len(status.Diagnosis.Causes) != 1 || status.Diagnosis.Causes[0].Code != "credential_invalid" || status.Diagnosis.Causes[0].LastError != "账号未登录" {
		t.Fatalf("unexpected invalid credential cause: %#v", status.Diagnosis.Causes)
	}
	if len(status.Diagnosis.Actions) == 0 || status.Diagnosis.Actions[0].Kind != "open_accounts" || status.Diagnosis.Actions[0].Target == nil || *status.Diagnosis.Actions[0].Target != "/third-party-accounts" {
		t.Fatalf("unexpected invalid credential actions: %#v", status.Diagnosis.Actions)
	}
}

func TestSourcePublishStatusIncludesCredentialDiagnosis(t *testing.T) {
	t.Parallel()

	var published []Status
	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 25, 0, 0, time.UTC), nil)
	source.notifyStatus = func(status Status) {
		published = append(published, status)
	}
	ctx := context.Background()
	checkedAt := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	if _, err := source.accounts.Upsert(ctx, thirdparty.UpsertRequest{
		Platform:  thirdparty.PlatformBilibili,
		AccountID: "primary",
		Label:     "主账号",
		Enabled:   true,
		Cookie:    "SESSDATA=invalid;",
		Credential: thirdparty.CredentialStatus{
			State:     thirdparty.CredentialInvalid,
			CheckedAt: &checkedAt,
			LastError: "账号未登录",
		},
	}); err != nil {
		t.Fatalf("upsert account: %v", err)
	}

	source.updateWatchCounts(ctx, map[string]Subject{
		"123456": {
			UID:      "123456",
			Services: map[string]bool{"live": true, "video": true},
		},
	})

	if len(published) == 0 {
		t.Fatal("expected status publication")
	}
	last := published[len(published)-1]
	if last.Diagnosis.Level != "action_required" || last.Diagnosis.Headline != "CK 需要重新登录" {
		t.Fatalf("unexpected published diagnosis: %#v", last.Diagnosis)
	}
	if len(last.Accounts) != 1 || last.Accounts[0].Credential.State != thirdparty.CredentialInvalid {
		t.Fatalf("published status should include current account state: %#v", last.Accounts)
	}
}

func TestSourceDiagnosisReportsHealthyState(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 26, 0, 0, time.UTC), nil)
	ctx := context.Background()
	source.updateWatchCounts(ctx, map[string]Subject{
		"123456": {
			UID:      "123456",
			Services: map[string]bool{"video": true},
		},
	})
	source.mu.Lock()
	now := time.Date(2026, 6, 8, 8, 26, 0, 0, time.UTC)
	source.status.Dynamic.LastPollAt = &now
	source.status.Status = source.deriveStateLocked()
	source.status.Summary = sourceSummary(source.status.Status)
	source.mu.Unlock()

	status := source.Status(ctx)
	if status.Status != StateConnected || status.Diagnosis.Level != "normal" || status.Diagnosis.Headline != "Bilibili 事件源运行中" {
		t.Fatalf("unexpected healthy diagnosis: %#v", status)
	}
	if !containsText(status.Diagnosis.Impacts, "动态接收不受影响。") {
		t.Fatalf("healthy diagnosis should explain dynamic state: %#v", status.Diagnosis.Impacts)
	}
}

func TestUpdateWatchCountsIgnoresUnwatchedRoomState(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 13, 0, 0, time.UTC), nil)
	ctx := context.Background()
	source.stateStore.SetRoom(ctx, sourceRoom{
		UID:             "old",
		ConnectionState: StateFailed,
		UpdatedAt:       time.Date(2026, 6, 8, 8, 12, 0, 0, time.UTC),
	})
	source.stateStore.SetRoom(ctx, sourceRoom{
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

	account, cookie, err := source.accountUsage.PrimaryCookie(ctx)
	if err != nil {
		t.Fatalf("PrimaryCookie: %v", err)
	}
	if account.AccountID != "valid" || !strings.Contains(cookie, "SESSDATA=valid;") {
		t.Fatalf("unexpected primary account %q cookie %q", account.AccountID, cookie)
	}
}

func TestRequestJSONIncludesHTTPStatusAndResponseBodyInErrors(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 15, 0, 0, time.UTC), func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":-352,"message":"风控校验失败","data":null}`)),
			Request:    request,
		}, nil
	})

	var document map[string]any
	err := source.requestJSON(context.Background(), http.MethodGet, bilibiliDynamic.FeedURL, "SESSDATA=fixture;", nil, &document)

	if err == nil {
		t.Fatalf("expected requestJSON error")
	}
	text := err.Error()
	for _, want := range []string{"risk_control", "code -352", "风控校验失败", "HTTP 200"} {
		if !strings.Contains(text, want) {
			t.Fatalf("requestJSON error missing %q: %s", want, text)
		}
	}
}

func TestRequestJSONIncludesHTTPFailureBody(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 16, 0, 0, time.UTC), func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusPreconditionFailed,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"code":-412,"message":"请求被拦截"}`)),
			Request:    request,
		}, nil
	})

	var document map[string]any
	err := source.requestJSON(context.Background(), http.MethodGet, bilibiliDynamic.FeedURL, "SESSDATA=fixture;", nil, &document)

	if err == nil {
		t.Fatalf("expected requestJSON error")
	}
	text := err.Error()
	for _, want := range []string{"risk_control", "HTTP 412", `"code":-412`, "请求被拦截"} {
		if !strings.Contains(text, want) {
			t.Fatalf("requestJSON error missing %q: %s", want, text)
		}
	}
}

func TestRequestJSONWithoutTargetStillChecksBilibiliCode(t *testing.T) {
	t.Parallel()

	source, _ := newTestSource(t, time.Date(2026, 6, 8, 8, 17, 0, 0, time.UTC), func(request *http.Request) (*http.Response, error) {
		return jsonResponse(`{"code":-111,"message":"csrf 校验失败"}`), nil
	})

	err := source.requestJSON(context.Background(), http.MethodPost, bilibiliDynamic.FollowURL, "SESSDATA=fixture; bili_jct=csrf;", strings.NewReader("csrf=csrf"), nil)

	if err == nil {
		t.Fatalf("expected requestJSON error")
	}
	text := err.Error()
	for _, want := range []string{"code -111", "csrf 校验失败"} {
		if !strings.Contains(text, want) {
			t.Fatalf("requestJSON error missing %q: %s", want, text)
		}
	}
}

func newTestSource(t *testing.T, now time.Time, handler func(*http.Request) (*http.Response, error)) (*Source, *dispatchRecorder) {
	t.Helper()
	return newTestSourceWithPluginConfig(t, now, handler, staticPluginConfig{})
}

func newTestSourceWithPluginConfig(t *testing.T, now time.Time, handler func(*http.Request) (*http.Response, error), pluginConfig staticPluginConfig) (*Source, *dispatchRecorder) {
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
		Store:         Store{Read: store.Read, Write: store.Write},
		Accounts:      accounts,
		Subjects:      bilibilisubscriptions.NewPluginConfigProvider(pluginConfig),
		Dispatcher:    recorder,
		HTTPTransport: roundTripFunc(handler),
		Now:           func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewSource: %v", err)
	}
	return source, recorder
}
