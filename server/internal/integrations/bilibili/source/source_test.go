package source

import (
	"context"
	bilibiliLive "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/live"
	bilibilimonitoring "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/monitoring"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"net/http"
	"strings"
	"testing"
	"time"
)

const monitorCookie = "SESSDATA=fixture; bili_jct=csrf; buvid3=buvid3; buvid4=buvid4; b_nut=1780905600; bili_ticket=ticket-value; bili_ticket_expires=4102444800; buvid_fp=fp; _uuid=uuid;"

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
	item := bilibiliLive.StatusItem{
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
	if started.EventType != EventLiveStarted {
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

	source, recorder := newTestSourceWithPluginConfig(t, time.Date(2026, 6, 8, 8, 11, 0, 0, time.UTC), func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "api.bilibili.com/bapis/bilibili.api.ticket.v1.Ticket/GenWebTicket":
			return jsonResponse(`{
				"code": 0,
				"data": {
					"ticket": "ticket-value",
					"created_at": 1780906260,
					"ttl": 259200,
					"nav": {
						"img": "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png",
						"sub": "https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png"
					}
				}
			}`), nil
		case "api.bilibili.com/x/polymer/web-dynamic/v1/feed/all":
			if request.URL.Query().Get("wts") != "1780906260" || request.URL.Query().Get("w_rid") == "" {
				t.Fatalf("dynamic feed url missing WBI signature: %s", request.URL.String())
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
					"services": []any{"video"},
				},
			},
		},
	})

	subjects := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "测试 UP",
			Services: map[string]bool{"video": true},
		},
	}
	ctx := context.Background()
	source.markSeen(ctx, EventDynamicPublished+":baseline", "123456", EventDynamicPublished, "baseline")
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
	snapshots := source.stateStore.LoadDynamics(ctx)
	monitor := snapshots["123456"].MonitorDynamic()
	if monitor == nil || monitor.LastID != "90001" || monitor.Title != "新视频标题" {
		t.Fatalf("unexpected monitor dynamic snapshot: %#v", monitor)
	}
}

func TestDynamicEventVideoURLUsesArchiveIDOrDynamicPage(t *testing.T) {
	t.Parallel()

	watched := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "测试 UP",
			Services: map[string]bool{"video": true},
		},
	}

	withBVID, ok := bilibilimonitoring.DynamicEventFromItem(map[string]any{
		"id_str": "100000000000000004",
		"type":   "DYNAMIC_TYPE_AV",
		"modules": map[string]any{
			"module_author": map[string]any{"mid": "123456", "name": "测试 UP"},
			"module_dynamic": map[string]any{
				"major": map[string]any{
					"type":    "MAJOR_TYPE_ARCHIVE",
					"archive": map[string]any{"title": "测试视频号", "bvid": "BV1RayleaBot"},
				},
			},
		},
	}, watched)
	if !ok || withBVID.URL != "https://www.bilibili.com/video/BV1RayleaBot" {
		t.Fatalf("dynamic video url with bvid = %#v, ok=%v", withBVID, ok)
	}

	withoutArchiveID, ok := bilibilimonitoring.DynamicEventFromItem(map[string]any{
		"id_str": "100000000000000004",
		"type":   "DYNAMIC_TYPE_AV",
		"modules": map[string]any{
			"module_author": map[string]any{"mid": "123456", "name": "测试 UP"},
			"module_dynamic": map[string]any{
				"major": map[string]any{
					"type":    "MAJOR_TYPE_ARCHIVE",
					"archive": map[string]any{"title": "缺少测试视频号"},
				},
			},
		},
	}, watched)
	if !ok || withoutArchiveID.URL != "https://t.bilibili.com/100000000000000004/" {
		t.Fatalf("dynamic video url without archive id = %#v, ok=%v", withoutArchiveID, ok)
	}
}

func TestDynamicEventOpusImageTextIncludesRichSummaryAndSingleImage(t *testing.T) {
	t.Parallel()

	watched := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "测试 UP",
			Services: map[string]bool{"image_text": true},
		},
	}

	event, ok := bilibilimonitoring.DynamicEventFromItem(map[string]any{
		"id_str": "100000000000000002",
		"type":   "DYNAMIC_TYPE_DRAW",
		"modules": map[string]any{
			"module_author": map[string]any{
				"mid":    "123456",
				"name":   "测试 UP",
				"pub_ts": float64(1781250000),
			},
			"module_dynamic": map[string]any{
				"topic": map[string]any{
					"id":       float64(1156147),
					"name":     "测试 UP2026巡演",
					"jump_url": "//m.bilibili.com/topic-detail?topic_id=1156147",
				},
				"major": map[string]any{
					"type": "MAJOR_TYPE_OPUS",
					"opus": map[string]any{
						"jump_url": "https://www.bilibili.com/opus/100000000000000002",
						"summary": map[string]any{
							"text": "#测试活动 2026#\n线下演唱会，测试内容更新。[打call]",
							"rich_text_nodes": []any{
								map[string]any{"type": "RICH_TEXT_NODE_TYPE_WEB", "text": "#测试活动 2026#"},
								map[string]any{"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": "\n线下演唱会，测试内容更新。"},
								map[string]any{
									"type": "RICH_TEXT_NODE_TYPE_TEXT",
									"text": "[打call]",
									"emoji": map[string]any{
										"text":     "[打call]",
										"icon_url": "//i0.hdslb.com/bfs/emote/call.png",
									},
								},
							},
						},
						"pics": []any{
							map[string]any{
								"url":    "//i0.hdslb.com/bfs/new_dyn/single.jpg",
								"width":  float64(900),
								"height": float64(1600),
							},
						},
					},
				},
			},
		},
	}, watched)

	if !ok {
		t.Fatal("expected opus image_text dynamic event")
	}
	if event.Service != "image_text" || event.Summary == "" || !strings.Contains(event.Summary, "线下演唱会") {
		t.Fatalf("unexpected opus summary: %#v", event)
	}
	if event.Topic == nil || event.Topic.ID != 1156147 || event.Topic.Name != "测试 UP2026巡演" || event.Topic.JumpURL != "https://m.bilibili.com/topic-detail?topic_id=1156147" {
		t.Fatalf("unexpected opus topic: %#v", event.Topic)
	}
	if !strings.Contains(event.SummaryHTML, "#测试 UP2026巡演#") || !strings.Contains(event.SummaryHTML, "rich-text-topic") || !strings.Contains(event.SummaryHTML, "rich-text-emoji") || !strings.Contains(event.SummaryHTML, "https://i0.hdslb.com/bfs/emote/call.png") {
		t.Fatalf("unexpected opus summary html: %q", event.SummaryHTML)
	}
	if event.URL != "https://www.bilibili.com/opus/100000000000000002" {
		t.Fatalf("unexpected opus url: %q", event.URL)
	}
	if len(event.Images) != 1 || event.Images[0].URL != "https://i0.hdslb.com/bfs/new_dyn/single.jpg" || event.Images[0].Width != 900 || event.Images[0].Height != 1600 {
		t.Fatalf("unexpected opus images: %#v", event.Images)
	}
}

func TestDynamicEventRepostIncludesOriginalRichTextAndImages(t *testing.T) {
	t.Parallel()

	watched := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "转发 UP",
			Services: map[string]bool{"repost": true},
		},
	}

	event, ok := bilibilimonitoring.DynamicEventFromItem(map[string]any{
		"id_str": "100000000000000005",
		"type":   "DYNAMIC_TYPE_FORWARD",
		"modules": map[string]any{
			"module_author": map[string]any{
				"mid":    "123456",
				"name":   "转发 UP",
				"pub_ts": float64(1781240000),
			},
			"module_dynamic": map[string]any{
				"topic": map[string]any{
					"id":       float64(10001),
					"name":     "星穹铁道",
					"jump_url": "https://m.bilibili.com/topic-detail?topic_id=10001",
				},
				"desc": map[string]any{
					"rich_text_nodes": []any{
						map[string]any{"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": "转发说明 "},
						map[string]any{"type": "RICH_TEXT_NODE_TYPE_EMOJI", "text": "[OK]", "emoji": map[string]any{"text": "[OK]", "icon_url": "//i0.hdslb.com/bfs/emote/ok.png"}},
					},
				},
			},
		},
		"orig": map[string]any{
			"id_str": "100000000000000006",
			"type":   "DYNAMIC_TYPE_DRAW",
			"modules": map[string]any{
				"module_author": map[string]any{
					"mid":    "654321",
					"name":   "原作者",
					"pub_ts": float64(1781230000),
				},
				"module_dynamic": map[string]any{
					"topic": map[string]any{
						"id":       float64(10001),
						"name":     "星穹铁道",
						"jump_url": "https://m.bilibili.com/topic-detail?topic_id=10001",
					},
					"desc": map[string]any{
						"rich_text_nodes": []any{
							map[string]any{"type": "RICH_TEXT_NODE_TYPE_TOPIC", "text": "#星穹铁道#"},
							map[string]any{"type": "RICH_TEXT_NODE_TYPE_TEXT", "text": " 原动态正文 "},
							map[string]any{"type": "RICH_TEXT_NODE_TYPE_WEB", "text": "互动抽奖", "jump_url": "https://www.bilibili.com/blackboard/activity-lottery.html"},
						},
					},
					"major": map[string]any{
						"type": "MAJOR_TYPE_OPUS",
						"opus": map[string]any{
							"pics": []any{
								map[string]any{"url": "//i0.hdslb.com/bfs/new_dyn/original-a.jpg", "width": float64(800), "height": float64(800)},
								map[string]any{"url": "//i0.hdslb.com/bfs/new_dyn/original-b.jpg", "width": float64(900), "height": float64(1200)},
							},
						},
					},
				},
			},
		},
	}, watched)

	if !ok {
		t.Fatal("expected repost dynamic event")
	}
	if event.Service != "repost" || event.Original == nil {
		t.Fatalf("unexpected repost event: %#v", event)
	}
	if !strings.Contains(event.SummaryHTML, "rich-text-emoji") {
		t.Fatalf("unexpected repost summary html: %q", event.SummaryHTML)
	}
	if event.Original.Author.UID != "654321" || event.Original.Service != "image_text" {
		t.Fatalf("unexpected original identity: %#v", event.Original)
	}
	if event.Original.Topic == nil || event.Original.Topic.ID != 10001 || event.Original.Topic.Name != "星穹铁道" {
		t.Fatalf("unexpected original topic: %#v", event.Original.Topic)
	}
	if !strings.Contains(event.Original.SummaryHTML, "rich-text-topic") || !strings.Contains(event.Original.SummaryHTML, "rich-text-lottery") {
		t.Fatalf("unexpected original summary html: %q", event.Original.SummaryHTML)
	}
	if len(event.Original.Images) != 2 || event.Original.Images[1].Height != 1200 {
		t.Fatalf("unexpected original images: %#v", event.Original.Images)
	}
}

func TestPollDynamicsBootstrapsExistingUpdatesBeforeDispatch(t *testing.T) {
	t.Parallel()

	call := 0
	source, recorder := newTestSourceWithPluginConfig(t, time.Date(2026, 6, 8, 8, 21, 0, 0, time.UTC), func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "api.bilibili.com/bapis/bilibili.api.ticket.v1.Ticket/GenWebTicket":
			return jsonResponse(`{
				"code": 0,
				"data": {
					"ticket": "ticket-value",
					"created_at": 1780906860,
					"ttl": 259200,
					"nav": {
						"img": "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png",
						"sub": "https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png"
					}
				}
			}`), nil
		case "api.bilibili.com/x/polymer/web-dynamic/v1/feed/all":
			call++
			items := `{
				"id_str": "old-90001",
				"type": "DYNAMIC_TYPE_AV",
				"basic": {"jump_url": "//www.bilibili.com/video/BV1OLD"},
				"modules": {
					"module_author": {"mid": "123456", "name": "测试 UP", "pub_ts": 1780646400},
					"module_dynamic": {
						"major": {
							"type": "MAJOR_TYPE_ARCHIVE",
							"archive": {"title": "几天前的视频", "desc": "历史内容", "cover": "//i0.hdslb.com/bfs/archive/old.jpg"}
						}
					}
				}
			}`
			if call > 1 {
				items = `{
					"id_str": "new-90002",
					"type": "DYNAMIC_TYPE_AV",
					"basic": {"jump_url": "//www.bilibili.com/video/BV1NEW"},
					"modules": {
						"module_author": {"mid": "123456", "name": "测试 UP", "pub_ts": 1780906800},
						"module_dynamic": {
							"major": {
								"type": "MAJOR_TYPE_ARCHIVE",
								"archive": {"title": "新视频", "desc": "新内容", "cover": "//i0.hdslb.com/bfs/archive/new.jpg"}
							}
						}
					}
				},` + items
			}
			return jsonResponse(`{"code":0,"data":{"items":[` + items + `]}}`), nil
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
					"services": []any{"video"},
				},
			},
		},
	})
	ctx := context.Background()
	subjects := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "测试 UP",
			Services: map[string]bool{"video": true},
		},
	}
	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}

	source.pollDynamics(ctx, subjects, account, "SESSDATA=fixture; bili_jct=csrf;")
	if len(recorder.events) != 0 {
		t.Fatalf("first dynamic poll dispatched history: %#v", recorder.events)
	}

	source.pollDynamics(ctx, subjects, account, "SESSDATA=fixture; bili_jct=csrf;")
	if len(recorder.events) != 1 {
		t.Fatalf("second dynamic poll events = %d, want 1", len(recorder.events))
	}
	payload := bilibiliPayload(t, recorder.events[0])
	if payload["id"] != "new-90002" || payload["title"] != "新视频" {
		t.Fatalf("unexpected dispatched dynamic payload: %#v", payload)
	}
}

func TestPollDynamicsDispatchesAfterEmptyBootstrap(t *testing.T) {
	t.Parallel()

	call := 0
	source, recorder := newTestSourceWithPluginConfig(t, time.Date(2026, 6, 8, 8, 22, 0, 0, time.UTC), func(request *http.Request) (*http.Response, error) {
		switch request.URL.Host + request.URL.Path {
		case "api.bilibili.com/bapis/bilibili.api.ticket.v1.Ticket/GenWebTicket":
			return jsonResponse(`{
				"code": 0,
				"data": {
					"ticket": "ticket-value",
					"created_at": 1780906920,
					"ttl": 259200,
					"nav": {
						"img": "https://i0.hdslb.com/bfs/wbi/7cd084941338484aae1ad9425b84077c.png",
						"sub": "https://i0.hdslb.com/bfs/wbi/4932caff0ff746eab6f01bf08b70ac45.png"
					}
				}
			}`), nil
		case "api.bilibili.com/x/polymer/web-dynamic/v1/feed/all":
			call++
			if call == 1 {
				return jsonResponse(`{"code":0,"data":{"items":[]}}`), nil
			}
			return jsonResponse(`{"code":0,"data":{"items":[{
				"id_str": "new-after-empty",
				"type": "DYNAMIC_TYPE_AV",
				"basic": {"jump_url": "//www.bilibili.com/video/BV1AFTEREMPTY"},
				"modules": {
					"module_author": {"mid": "123456", "name": "测试 UP", "pub_ts": 1780906900},
					"module_dynamic": {
						"major": {
							"type": "MAJOR_TYPE_ARCHIVE",
							"archive": {"title": "空基线后的新视频", "desc": "新内容", "cover": "//i0.hdslb.com/bfs/archive/after-empty.jpg"}
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
					"services": []any{"video"},
				},
			},
		},
	})
	ctx := context.Background()
	subjects := map[string]Subject{
		"123456": {
			UID:      "123456",
			Name:     "测试 UP",
			Services: map[string]bool{"video": true},
		},
	}
	account := thirdparty.Account{Platform: thirdparty.PlatformBilibili, AccountID: "primary"}

	source.pollDynamics(ctx, subjects, account, "SESSDATA=fixture; bili_jct=csrf;")
	source.pollDynamics(ctx, subjects, account, "SESSDATA=fixture; bili_jct=csrf;")

	if len(recorder.events) != 1 {
		t.Fatalf("dynamic events = %d, want 1", len(recorder.events))
	}
	payload := bilibiliPayload(t, recorder.events[0])
	if payload["id"] != "new-after-empty" {
		t.Fatalf("unexpected dynamic payload: %#v", payload)
	}
}

func TestMonitorSnapshotMergesSubjectsRoomsAndDynamicSnapshots(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 20, 0, 0, time.UTC)
	source, _ := newTestSourceWithPluginConfig(t, now, monitorDynamicTransport(t, now, `{
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
	}`), staticPluginConfig{
		values: map[string]any{
			"subscriptions": []any{
				map[string]any{
					"enabled":    true,
					"platform":   "bilibili",
					"uid":        "123456",
					"name":       "订阅名",
					"avatar_url": "https://i0.hdslb.com/bfs/face/subject.jpg",
					"services":   []any{"live", "video"},
				},
			},
		},
	})
	ctx := context.Background()
	seedBilibiliAccount(t, source, ctx)
	eventTime := time.Date(2026, 6, 8, 8, 19, 30, 0, time.UTC)
	source.stateStore.SetRoom(ctx, sourceRoom{
		UID:             "123456",
		RoomID:          "10001",
		Name:            "直播间标题",
		Face:            "https://i0.hdslb.com/bfs/face/live.jpg",
		CoverURL:        "https://i0.hdslb.com/bfs/live/cover.jpg",
		LiveStatus:      1,
		LiveStartedAt:   1780906000,
		ConnectionState: StateConnected,
		LastEventAt:     &eventTime,
		UpdatedAt:       now,
	})

	snapshot, err := source.MonitorSnapshot(ctx)
	if err != nil {
		t.Fatalf("MonitorSnapshot: %v", err)
	}
	if snapshot.Platform != thirdparty.PlatformBilibili || len(snapshot.Items) != 1 {
		t.Fatalf("unexpected monitor snapshot: %#v", snapshot)
	}
	item := snapshot.Items[0]
	if item.UID != "123456" || item.Username != "直播间标题" || item.AvatarURL != "https://i0.hdslb.com/bfs/face/live.jpg" {
		t.Fatalf("unexpected monitor identity: %#v", item)
	}
	if item.ProfileURL != "https://space.bilibili.com/123456/" {
		t.Fatalf("profile url = %q", item.ProfileURL)
	}
	if strings.Join(item.Services, ",") != "live,video" {
		t.Fatalf("unexpected monitor services: %#v", item.Services)
	}
	if item.Dynamic == nil || item.Dynamic.LastID != "90001" || len(item.Dynamic.Images) != 1 || item.Dynamic.Images[0].URL != "https://i0.hdslb.com/bfs/archive/cover.jpg" {
		t.Fatalf("unexpected monitor dynamic: %#v", item.Dynamic)
	}
	if !item.Live.IsLive || item.Live.RoomID != "10001" || item.Live.CoverURL != "https://i0.hdslb.com/bfs/live/cover.jpg" || item.Live.LiveStartedAt == nil {
		t.Fatalf("unexpected monitor live: %#v", item.Live)
	}
}

func TestMonitorSnapshotRefreshesLatestDynamicWithoutDispatch(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 24, 0, 0, time.UTC)
	source, recorder := newTestSourceWithPluginConfig(t, now, monitorDynamicTransport(t, now, `{
		"id_str": "100000000000000004",
		"type": "DYNAMIC_TYPE_AV",
		"modules": {
			"module_author": {"mid": "123456", "name": "测试 UP", "pub_ts": 1546300800},
			"module_dynamic": {
				"major": {
					"type": "MAJOR_TYPE_ARCHIVE",
					"archive": {
						"title": "很久前的最新视频",
						"desc": "仍然是主页最新动态",
						"cover": "//i0.hdslb.com/bfs/archive/latest.jpg"
					}
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
	source.stateStore.SetDynamic(ctx, BilibiliEvent{
		UID:     "123456",
		ID:      "old-90001",
		Service: "video",
		Title:   "几天前的视频",
		URL:     "https://www.bilibili.com/video/BV1OLD",
		PubTS:   1780646400,
		Author:  Author{Name: "测试 UP"},
	})

	snapshot, err := source.MonitorSnapshot(ctx)
	if err != nil {
		t.Fatalf("MonitorSnapshot: %v", err)
	}
	if len(recorder.events) != 0 {
		t.Fatalf("monitor dynamic refresh dispatched plugin events: %#v", recorder.events)
	}
	if len(snapshot.Items) != 1 || snapshot.Items[0].Dynamic == nil {
		t.Fatalf("unexpected monitor snapshot: %#v", snapshot)
	}
	dynamic := snapshot.Items[0].Dynamic
	if dynamic.LastID != "100000000000000004" || dynamic.Title != "很久前的最新视频" {
		t.Fatalf("unexpected latest dynamic: %#v", dynamic)
	}
	if dynamic.URL != "https://t.bilibili.com/100000000000000004/" {
		t.Fatalf("dynamic url = %q", dynamic.URL)
	}
	if dynamic.PublishedAt == nil || !dynamic.PublishedAt.Equal(time.Unix(1546300800, 0).UTC()) {
		t.Fatalf("published_at = %#v", dynamic.PublishedAt)
	}
}

func TestMonitorSnapshotSkipsPinnedDynamicWhenItIsNotLatest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 27, 0, 0, time.UTC)
	source, _ := newTestSourceWithPluginConfig(t, now, monitorDynamicTransport(t, now, `{
		"id_str": "100000000000000007",
		"type": "DYNAMIC_TYPE_AV",
		"modules": {
			"module_tag": {"text": "置顶"},
			"module_author": {"mid": "123456", "name": "测试 UP", "pub_ts": 1780800000},
			"module_dynamic": {
				"major": {
					"type": "MAJOR_TYPE_ARCHIVE",
					"archive": {"title": "置顶旧视频"}
				}
			}
		}
	}, {
		"id_str": "100000000000000004",
		"type": "DYNAMIC_TYPE_DRAW",
		"modules": {
			"module_author": {"mid": "123456", "name": "测试 UP", "pub_ts": 1780906200},
			"module_dynamic": {
				"desc": {"text": "真实最新图文"},
				"major": {
					"type": "MAJOR_TYPE_DRAW",
					"draw": {"items": [{"src": "//i0.hdslb.com/bfs/album/latest.jpg"}]}
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
	if dynamic.LastID != "100000000000000004" || dynamic.Title != "图文动态更新" || dynamic.Summary != "真实最新图文" {
		t.Fatalf("unexpected latest dynamic after pinned item: %#v", dynamic)
	}
}

func TestMonitorSnapshotSkipsPinnedDynamicWithoutTimestamp(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 8, 8, 27, 30, 0, time.UTC)
	source, _ := newTestSourceWithPluginConfig(t, now, monitorDynamicTransport(t, now, `{
		"id_str": "100000000000000007",
		"type": "DYNAMIC_TYPE_AV",
		"modules": {
			"module_tag": {"text": "置顶"},
			"module_author": {"mid": "123456", "name": "测试 UP"},
			"module_dynamic": {
				"major": {
					"type": "MAJOR_TYPE_ARCHIVE",
					"archive": {"title": "置顶无时间"}
				}
			}
		}
	}, {
		"id_str": "100000000000000009",
		"type": "DYNAMIC_TYPE_DRAW",
		"modules": {
			"module_author": {"mid": "123456", "name": "测试 UP"},
			"module_dynamic": {
				"desc": {"text": "非置顶无时间"},
				"major": {
					"type": "MAJOR_TYPE_DRAW",
					"draw": {"items": [{"src": "//i0.hdslb.com/bfs/album/latest.jpg"}]}
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
	if dynamic.LastID != "100000000000000009" || dynamic.Summary != "非置顶无时间" {
		t.Fatalf("unexpected dynamic without timestamps: %#v", dynamic)
	}
}
