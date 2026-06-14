package shell

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestEnrichEventMetadataHydratesGroupContextAndUsesCache(t *testing.T) {
	t.Parallel()

	var groupInfoCalls atomic.Int32
	var memberInfoCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var request apiCallRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		switch request.Action {
		case "get_group_info":
			groupInfoCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":  "ok",
				"retcode": 0,
				"data": map[string]any{
					"group_name": "测试群",
				},
			})
		case "get_group_member_info":
			memberInfoCalls.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":  "ok",
				"retcode": 0,
				"data": map[string]any{
					"nickname": "普通昵称",
					"card":     "群名片",
					"role":     "member",
				},
			})
		default:
			t.Fatalf("unexpected action: %s", request.Action)
		}
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		HTTPAPI: config.OneBotTransportConfig{
			Enabled: true,
			URL:     server.URL,
		},
	}, shellDeps{
		connectTimeout: 500 * time.Millisecond,
		sleep:          blockingSleep,
	})

	event := NormalizedEvent{
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		EventType:        "message.group",
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "30001",
		PayloadFields: map[string]any{
			"sender": map[string]any{},
			"onebot": map[string]any{
				"self_id":      "10001",
				"group_id":     "20001",
				"user_id":      "30001",
				"message_type": "group",
				"sender":       map[string]any{},
			},
		},
	}

	enriched := shell.EnrichEventMetadata(context.Background(), event)
	if enriched.TargetName != "测试群" {
		t.Fatalf("unexpected target name: %#v", enriched.TargetName)
	}
	if enriched.ActorNickname != "群名片" {
		t.Fatalf("unexpected actor nickname: %#v", enriched.ActorNickname)
	}
	if enriched.ActorRole != "member" {
		t.Fatalf("unexpected actor role: %#v", enriched.ActorRole)
	}

	sender := enriched.PayloadFields["sender"].(map[string]any)
	if sender["nickname"] != "普通昵称" || sender["card"] != "群名片" || sender["role"] != "member" {
		t.Fatalf("unexpected sender payload: %#v", sender)
	}

	enrichedAgain := shell.EnrichEventMetadata(context.Background(), event)
	if enrichedAgain.TargetName != "测试群" {
		t.Fatalf("unexpected cached target name: %#v", enrichedAgain.TargetName)
	}
	if groupInfoCalls.Load() != 1 {
		t.Fatalf("expected one group info lookup, got %d", groupInfoCalls.Load())
	}
	if memberInfoCalls.Load() != 1 {
		t.Fatalf("expected one member info lookup, got %d", memberInfoCalls.Load())
	}
}

func TestEnrichEventMetadataHydratesPrivateNicknameAndUsesCache(t *testing.T) {
	t.Parallel()

	var strangerInfoCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var request apiCallRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Action != "get_stranger_info" {
			t.Fatalf("unexpected action: %s", request.Action)
		}

		strangerInfoCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"nickname": "好友昵称",
			},
		})
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		HTTPAPI: config.OneBotTransportConfig{
			Enabled: true,
			URL:     server.URL,
		},
	}, shellDeps{
		connectTimeout: 500 * time.Millisecond,
		sleep:          blockingSleep,
	})

	event := NormalizedEvent{
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		EventType:        "message.private",
		ConversationType: "private",
		ConversationID:   "30002",
		SenderID:         "30002",
		PayloadFields: map[string]any{
			"sender": map[string]any{},
			"onebot": map[string]any{
				"self_id":      "10001",
				"user_id":      "30002",
				"message_type": "private",
				"sender":       map[string]any{},
			},
		},
	}

	enriched := shell.EnrichEventMetadata(context.Background(), event)
	if enriched.ActorNickname != "好友昵称" {
		t.Fatalf("unexpected actor nickname: %#v", enriched.ActorNickname)
	}
	if enriched.TargetName != "" {
		t.Fatalf("private event should not set target name: %#v", enriched.TargetName)
	}

	sender := enriched.PayloadFields["sender"].(map[string]any)
	if sender["nickname"] != "好友昵称" {
		t.Fatalf("unexpected sender payload: %#v", sender)
	}

	_ = shell.EnrichEventMetadata(context.Background(), event)
	if strangerInfoCalls.Load() != 1 {
		t.Fatalf("expected one stranger info lookup, got %d", strangerInfoCalls.Load())
	}
}

func TestEnrichEventMetadataRefreshesGroupNameAfterNotice(t *testing.T) {
	t.Parallel()

	names := []string{"旧群名", "新群名"}
	var groupInfoCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var request apiCallRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Action != "get_group_info" {
			t.Fatalf("unexpected action: %s", request.Action)
		}

		call := int(groupInfoCalls.Add(1))
		name := names[min(call-1, len(names)-1)]
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"group_name": name,
			},
		})
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		HTTPAPI: config.OneBotTransportConfig{
			Enabled: true,
			URL:     server.URL,
		},
	}, shellDeps{
		connectTimeout: 500 * time.Millisecond,
		sleep:          blockingSleep,
	})

	event := NormalizedEvent{
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		EventType:        "message.group",
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "30001",
		PayloadFields: map[string]any{
			"sender": map[string]any{"nickname": "用户"},
			"onebot": map[string]any{
				"group_id": "20001",
				"user_id":  "30001",
				"sender":   map[string]any{"nickname": "用户"},
			},
		},
	}

	if enriched := shell.EnrichEventMetadata(context.Background(), event); enriched.TargetName != "旧群名" {
		t.Fatalf("unexpected first target name: %#v", enriched.TargetName)
	}

	shell.EnrichEventMetadata(context.Background(), NormalizedEvent{
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		EventType:        "notice.group_name",
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "30001",
		PayloadFields: map[string]any{
			"notice_type": "group_name",
			"onebot": map[string]any{
				"notice_type": "group_name",
				"group_id":    "20001",
			},
		},
	})

	enriched := shell.EnrichEventMetadata(context.Background(), event)
	if enriched.TargetName != "新群名" {
		t.Fatalf("unexpected refreshed target name: %#v", enriched.TargetName)
	}
	if groupInfoCalls.Load() != 2 {
		t.Fatalf("expected two group info lookups, got %d", groupInfoCalls.Load())
	}
}

func TestIdentityCacheRefreshesGroupNameAfterRawNoticeFrame(t *testing.T) {
	t.Parallel()

	names := []string{"旧群名", "新群名"}
	var groupInfoCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var request apiCallRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Action != "get_group_info" {
			t.Fatalf("unexpected action: %s", request.Action)
		}

		call := int(groupInfoCalls.Add(1))
		name := names[min(call-1, len(names)-1)]
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"group_name": name,
			},
		})
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		HTTPAPI: config.OneBotTransportConfig{
			Enabled: true,
			URL:     server.URL,
		},
	}, shellDeps{
		connectTimeout: 500 * time.Millisecond,
		sleep:          blockingSleep,
	})

	event := NormalizedEvent{
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		EventType:        "message.group",
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "30001",
		PayloadFields: map[string]any{
			"sender": map[string]any{"nickname": "用户"},
		},
	}

	if enriched := shell.EnrichEventMetadata(context.Background(), event); enriched.TargetName != "旧群名" {
		t.Fatalf("unexpected first target name: %#v", enriched.TargetName)
	}

	shell.invalidateIdentityCacheForFrame(oneBotFrame{
		PostType:   "notice",
		NoticeType: "group_name",
		GroupID:    20001,
		UserID:     30001,
	})

	enriched := shell.EnrichEventMetadata(context.Background(), event)
	if enriched.TargetName != "新群名" {
		t.Fatalf("unexpected refreshed target name: %#v", enriched.TargetName)
	}
	if groupInfoCalls.Load() != 2 {
		t.Fatalf("expected two group info lookups, got %d", groupInfoCalls.Load())
	}
}

func TestEnrichEventMetadataUsesMessageGroupNameOverCachedLookup(t *testing.T) {
	t.Parallel()

	var groupInfoCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var request apiCallRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Action != "get_group_info" {
			t.Fatalf("unexpected action: %s", request.Action)
		}

		groupInfoCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"group_name": "接口群名",
			},
		})
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		HTTPAPI: config.OneBotTransportConfig{
			Enabled: true,
			URL:     server.URL,
		},
	}, shellDeps{
		connectTimeout: 500 * time.Millisecond,
		sleep:          blockingSleep,
	})

	event := NormalizedEvent{
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		EventType:        "message.group",
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "30001",
		PayloadFields: map[string]any{
			"sender": map[string]any{"nickname": "用户"},
		},
	}

	if enriched := shell.EnrichEventMetadata(context.Background(), event); enriched.TargetName != "接口群名" {
		t.Fatalf("unexpected first target name: %#v", enriched.TargetName)
	}

	event.PayloadFields = map[string]any{
		"sender": map[string]any{"nickname": "用户"},
		"onebot": map[string]any{
			"group_name": "消息群名",
		},
	}

	enriched := shell.EnrichEventMetadata(context.Background(), event)
	if enriched.TargetName != "消息群名" {
		t.Fatalf("unexpected message target name: %#v", enriched.TargetName)
	}

	event.PayloadFields = map[string]any{
		"sender": map[string]any{"nickname": "用户"},
	}
	enriched = shell.EnrichEventMetadata(context.Background(), event)
	if enriched.TargetName != "消息群名" {
		t.Fatalf("unexpected cached target name from message: %#v", enriched.TargetName)
	}
	if groupInfoCalls.Load() != 1 {
		t.Fatalf("expected one group info lookup, got %d", groupInfoCalls.Load())
	}
}

func TestEnrichEventMetadataRefreshesMemberInfoAfterCardNotice(t *testing.T) {
	t.Parallel()

	members := []GroupMemberInfo{
		{Nickname: "旧昵称", Card: "旧名片", Role: "member", Title: "旧头衔"},
		{Nickname: "新昵称", Card: "新名片", Role: "admin", Title: "新头衔"},
	}
	var memberInfoCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var request apiCallRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Action != "get_group_member_info" {
			t.Fatalf("unexpected action: %s", request.Action)
		}

		call := int(memberInfoCalls.Add(1))
		info := members[min(call-1, len(members)-1)]
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"nickname": info.Nickname,
				"card":     info.Card,
				"role":     info.Role,
				"title":    info.Title,
			},
		})
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		HTTPAPI: config.OneBotTransportConfig{
			Enabled: true,
			URL:     server.URL,
		},
	}, shellDeps{
		connectTimeout: 500 * time.Millisecond,
		sleep:          blockingSleep,
	})

	event := NormalizedEvent{
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		EventType:        "message.group",
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "30001",
		TargetName:       "群",
		PayloadFields: map[string]any{
			"sender": map[string]any{},
			"onebot": map[string]any{
				"group_id": "20001",
				"user_id":  "30001",
				"sender":   map[string]any{},
			},
		},
	}

	first := shell.EnrichEventMetadata(context.Background(), event)
	firstSender := first.PayloadFields["sender"].(map[string]any)
	if first.ActorNickname != "旧名片" || first.ActorRole != "member" || firstSender["title"] != "旧头衔" {
		t.Fatalf("unexpected first sender: actor=%q role=%q sender=%#v", first.ActorNickname, first.ActorRole, firstSender)
	}

	shell.EnrichEventMetadata(context.Background(), NormalizedEvent{
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		EventType:        "notice.group_card",
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "30001",
		PayloadFields: map[string]any{
			"notice_type": "group_card",
			"onebot": map[string]any{
				"notice_type": "group_card",
				"group_id":    "20001",
				"user_id":     "30001",
			},
		},
	})

	refreshed := shell.EnrichEventMetadata(context.Background(), event)
	refreshedSender := refreshed.PayloadFields["sender"].(map[string]any)
	if refreshed.ActorNickname != "新名片" || refreshed.ActorRole != "admin" || refreshedSender["title"] != "新头衔" {
		t.Fatalf("unexpected refreshed sender: actor=%q role=%q sender=%#v", refreshed.ActorNickname, refreshed.ActorRole, refreshedSender)
	}
	if memberInfoCalls.Load() != 2 {
		t.Fatalf("expected two member info lookups, got %d", memberInfoCalls.Load())
	}
}
