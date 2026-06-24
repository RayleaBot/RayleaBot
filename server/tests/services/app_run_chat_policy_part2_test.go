package services

import (
	"context"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplyChatPolicyAppliesTargetLimitToCooldownReply(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	sender := &recordingOutboundSender{}
	limiter := &recordingAppOutboundLimiter{
		err: &adapteroutbound.Error{Code: "platform.rate_limited", Message: "outbound message rate limit exceeded"},
	}
	cfg := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		User: config.UserConfig{
			CommandRateLimit: "1/1h",
			CooldownReply:    true,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "5/1h",
		},
	}
	application := newTestAppState(cfg, logger)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "weather",
			Permission: "everyone",
		}},
	}}), nil, sender, bridge.New(logger, &recordingDispatcherClient{}))
	application.services.EventIngress.SetOutboundLimiter(limiter)

	event := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-weather-target-limit",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		TargetName:       "测试群",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/weather",
		MessageID:        "30001",
	}

	if _, allowed := application.applyChatPolicy(context.Background(), event); !allowed {
		t.Fatal("first command should be allowed")
	}
	if _, allowed := application.applyChatPolicy(context.Background(), event); allowed {
		t.Fatal("second command should be rate limited")
	}

	if sender.replyCount != 0 || sender.messageCount != 0 {
		t.Fatalf("rate limited cooldown reply should not send: replies=%d messages=%d", sender.replyCount, sender.messageCount)
	}
	request := limiter.lastRequest()
	if request.PluginID != "" || request.TargetType != "group" || request.TargetID != "20001" {
		t.Fatalf("unexpected limiter request: %#v", request)
	}
	summary := waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Details["error_code"] == "platform.rate_limited"
	})
	if summary.Level != "warn" || summary.Source != "adapter.onebot11" {
		t.Fatalf("unexpected rate limit summary: %+v", summary)
	}
}

func TestApplyChatPolicyCancelsCooldownReplyTargetLimit(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	limiter := &contextAwareOutboundLimiter{}
	cfg := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		User: config.UserConfig{
			CommandRateLimit: "1/1h",
			CooldownReply:    true,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "5/1h",
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "weather",
			Permission: "everyone",
		}},
	}}), nil, sender, nil)
	application.services.EventIngress.SetOutboundLimiter(limiter)

	event := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-weather-cancelled-limit",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/weather",
		MessageID:        "30001",
	}

	if _, allowed := application.applyChatPolicy(context.Background(), event); !allowed {
		t.Fatal("first command should be allowed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, allowed := application.applyChatPolicy(ctx, event); allowed {
		t.Fatal("second command should be rate limited")
	}
	if limiter.ctxErr != context.Canceled {
		t.Fatalf("limiter ctxErr = %v, want context.Canceled", limiter.ctxErr)
	}
	if sender.replyCount != 0 || sender.messageCount != 0 {
		t.Fatalf("cancelled cooldown reply should not send: replies=%d messages=%d", sender.replyCount, sender.messageCount)
	}
}

func TestApplyChatPolicyUsesCanonicalUserCooldownForPrivateCommand(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		User: config.UserConfig{
			CommandRateLimit: "1/1h",
			CooldownReply:    true,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "5/1h",
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "help",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "help",
		}},
	}}), nil, nil, nil)
	event := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-help-private-canonical",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        time.Now().Unix(),
		ConversationType: "private",
		ConversationID:   "10001",
		SenderID:         "10001",
		PlainText:        "/help",
		MessageID:        "40001",
	}

	if _, allowed := application.applyChatPolicy(context.Background(), event); !allowed {
		t.Fatal("first private command should be allowed")
	}
	if _, allowed := application.applyChatPolicy(context.Background(), event); allowed {
		t.Fatal("second private command should be blocked by canonical user cooldown")
	}
}

func TestApplyChatPolicyUsesCanonicalUserCooldownForGroupCommand(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		User: config.UserConfig{
			CommandRateLimit: "1/1h",
			CooldownReply:    true,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "5/1h",
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "weather",
		}},
	}}), nil, nil, nil)
	event := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-weather-group-user-canonical",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/weather",
		MessageID:        "40002",
	}

	if _, allowed := application.applyChatPolicy(context.Background(), event); !allowed {
		t.Fatal("first group command should be allowed")
	}
	deniedEvent := event
	deniedEvent.MessageID = "40003"
	if _, allowed := application.applyChatPolicy(context.Background(), deniedEvent); allowed {
		t.Fatal("second group command should be blocked by canonical user cooldown")
	}
}

func TestApplyChatPolicyUsesCanonicalGroupCooldown(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		User: config.UserConfig{
			CommandRateLimit: "5/1h",
			CooldownReply:    true,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "1/1h",
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "weather",
		}},
	}}), nil, nil, nil)
	firstEvent := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-weather-group-group-canonical-1",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/weather",
		MessageID:        "40004",
	}
	secondEvent := firstEvent
	secondEvent.EventID = "evt-weather-group-group-canonical-2"
	secondEvent.SenderID = "10003"
	secondEvent.MessageID = "40005"

	if _, allowed := application.applyChatPolicy(context.Background(), firstEvent); !allowed {
		t.Fatal("first group command should be allowed")
	}
	if _, allowed := application.applyChatPolicy(context.Background(), secondEvent); allowed {
		t.Fatal("second sender in same group should be blocked by canonical group cooldown")
	}
}

func TestApplyChatPolicyUsesCanonicalCooldownReplyFlag(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		User: config.UserConfig{
			CommandRateLimit: "1/1h",
			CooldownReply:    false,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "5/1h",
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "weather",
		}},
	}}), nil, sender, nil)
	event := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-weather-canonical-reply-flag",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/weather",
		MessageID:        "40006",
	}

	if _, allowed := application.applyChatPolicy(context.Background(), event); !allowed {
		t.Fatal("first group command should be allowed")
	}
	if _, allowed := application.applyChatPolicy(context.Background(), event); allowed {
		t.Fatal("second group command should be blocked by canonical cooldown")
	}
	if sender.replyCount != 0 || sender.messageCount != 0 {
		t.Fatalf("canonical cooldown reply flag should suppress replies: %+v", sender)
	}
}

func TestApplyChatPolicyUsesCanonicalPermissionAndSuperAdmin(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Admin: config.AdminConfig{
			SuperAdmins: []string{"42"},
		},
		Permission: config.PermissionConfig{
			DefaultLevel: "group_admin",
		},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		User: config.UserConfig{
			CommandRateLimit: "5/1h",
			CooldownReply:    true,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "5/1h",
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "ops",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "ops",
		}},
	}}), nil, nil, nil)

	memberEvent := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-ops-canonical-member",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/ops",
		MessageID:        "40007",
	}
	if _, allowed := application.applyChatPolicy(context.Background(), memberEvent); allowed {
		t.Fatal("member should be denied when canonical default level is group_admin")
	}

	superAdminEvent := memberEvent
	superAdminEvent.EventID = "evt-ops-canonical-super-admin"
	superAdminEvent.SenderID = "42"
	superAdminEvent.MessageID = "40008"
	if _, allowed := application.applyChatPolicy(context.Background(), superAdminEvent); !allowed {
		t.Fatal("canonical super admin should bypass permission checks")
	}
}

func TestHandleAdapterEventSendsBuiltinMenuImageWithoutPluginDispatch(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	sender := &recordingOutboundSender{}
	dispatcher := &recordingDispatcherClient{}
	runner := &captureRenderRunner{}
	renderRoot := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	application := newTestAppState(config.Config{
		Admin:   config.AdminConfig{SuperAdmins: []string{"10002"}},
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"help", "帮助"},
		}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}, logger)
	application.renderStack.Renderer = newRenderServiceForRepo(t, repoRoot, renderRoot, runner)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "天气",
		Description:       "查询天气",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:        "weather",
			Description: "查询城市天气",
			Usage:       "/weather 上海",
			Permission:  "everyone",
		}},
	}}), nil, sender, bridge.New(slog.Default(), dispatcher))

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-builtin-menu",
		BotID:            "10001",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		TargetName:       "测试群",
		SenderID:         "10002",
		ActorNickname:    "角色昵称",
		ActorRole:        "member",
		PlainText:        "/help",
		MessageID:        "30001",
		PayloadFields: map[string]any{
			"onebot": map[string]any{
				"user_id": "10002",
				"sender": map[string]any{
					"user_id":  "10002",
					"nickname": "普通昵称",
					"card":     "群名片",
					"role":     "member",
				},
			},
		},
	})

	if sender.replyCount != 1 || !strings.HasPrefix(sender.lastReplyImage, "file://") {
		t.Fatalf("unexpected builtin menu reply: count=%d image=%q", sender.replyCount, sender.lastReplyImage)
	}
	if dispatcher.deliverCount != 0 {
		t.Fatalf("builtin menu dispatched to plugins %d times", dispatcher.deliverCount)
	}
	summary := waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Message == "10001: [测试群(20001)]群名片/普通昵称(10002): /help"
	})
	if summary.Level != "info" || summary.Source != "bridge" || summary.Protocol != logging.ProtocolOneBot11 {
		t.Fatalf("unexpected builtin menu trigger log: %+v", summary)
	}
	if summary.Details["event_id"] != "evt-builtin-menu" || summary.Details["command_name"] != "help" {
		t.Fatalf("unexpected builtin menu trigger details: %#v", summary.Details)
	}
	if summary.Details["builtin_menu"] != true || summary.Details["plain_text"] != "/help" {
		t.Fatalf("unexpected builtin menu marker details: %#v", summary.Details)
	}
	summary = waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Source == "adapter.onebot11" && summary.Details["command_name"] == "help"
	})
	if summary.Level != "info" {
		t.Fatalf("unexpected builtin menu response level: %+v", summary)
	}
	if summary.Details["action_kind"] != "message.reply" || summary.Details["delivery_kind"] != "message.reply" {
		t.Fatalf("unexpected builtin menu response action details: %#v", summary.Details)
	}
	if summary.Details["plain_text"] != "[图片]" || summary.Details["message_id"] != "msg-2" {
		t.Fatalf("unexpected builtin menu response message details: %#v", summary.Details)
	}
	html := runner.lastHTML()
	for _, want := range []string{"群名片", "ID 10002", "测试群", "超级管理员", "nk=10002"} {
		if !strings.Contains(html, want) {
			t.Fatalf("builtin menu html missing sender identity field %q:\n%s", want, html)
		}
	}
	if strings.Contains(html, "访客") {
		t.Fatalf("builtin menu html should not fall back to guest identity:\n%s", html)
	}
}
