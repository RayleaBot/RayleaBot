package services

import (
	"context"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"log/slog"
	"reflect"
	"testing"
	"time"
)

func TestCommandInfoForEventUsesDefaultLevelForOmittedPermission(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "group_admin"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
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
			Name: "weather-admin",
		}},
	}}), nil, nil, nil)

	info := application.commandInfoForEvent(application.enrichCommandEvent(adapterintake.NormalizedEvent{
		PlainText: "/weather-admin",
	}))
	if info == nil {
		t.Fatal("commandInfoForEvent returned nil")
		return
	}
	if info.Permission != "group_admin" {
		t.Fatalf("permission = %q, want group_admin", info.Permission)
	}
}

func TestResolveChatPolicyConfigUsesConfiguredFields(t *testing.T) {
	t.Parallel()

	settings := chatpolicy.ResolveConfig(config.Config{
		Admin:      config.AdminConfig{SuperAdmins: []string{"canonical-admin"}},
		Permission: config.PermissionConfig{DefaultLevel: "group_admin"},
		User: config.UserConfig{
			CommandRateLimit: "2/1h",
			CooldownReply:    false,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "3/1h",
		},
	})

	if !reflect.DeepEqual(settings.SuperAdmins, []string{"canonical-admin"}) {
		t.Fatalf("unexpected super admins: %#v", settings.SuperAdmins)
	}
	if settings.DefaultLevel != "group_admin" {
		t.Fatalf("DefaultLevel = %q, want group_admin", settings.DefaultLevel)
	}
	if settings.UserCommandRateLimit != "2/1h" {
		t.Fatalf("UserCommandRateLimit = %q, want 2/1h", settings.UserCommandRateLimit)
	}
	if settings.GroupCommandRateLimit != "3/1h" {
		t.Fatalf("GroupCommandRateLimit = %q, want 3/1h", settings.GroupCommandRateLimit)
	}
	if settings.CooldownReplyEnabled {
		t.Fatal("CooldownReplyEnabled = true, want false")
	}
}

func TestHandleAdapterEventBlocksBlacklistedMessageBeforeBridge(t *testing.T) {
	t.Parallel()

	repo := newStubBlacklistRepo()
	repo.block("user", "bad-user")
	dispatcherClient := &recordingDispatcherClient{}
	application := newTestAppState(config.Config{}, nil)
	application.setTestEventIngress(nil, repo, nil, bridge.New(slog.Default(), dispatcherClient))

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-1",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        time.Now().Unix(),
		ConversationType: "private",
		ConversationID:   "10001",
		SenderID:         "bad-user",
		PlainText:        "hello",
	})

	if dispatcherClient.deliverCount != 0 {
		t.Fatalf("deliverCount = %d, want 0", dispatcherClient.deliverCount)
	}
}

func TestHandleAdapterEventKeepsBlacklistedNonCommandMessageSilent(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	repo := newStubBlacklistRepo()
	repo.block("user", "bad-user")
	dispatcherClient := &recordingDispatcherClient{}
	application := newTestAppState(config.Config{}, logger)
	application.setTestEventIngress(nil, repo, nil, bridge.New(logger, dispatcherClient))

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-blacklist-silent-1",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        time.Now().Unix(),
		ConversationType: "private",
		ConversationID:   "10001",
		SenderID:         "bad-user",
		PlainText:        "hello",
	})

	if dispatcherClient.deliverCount != 0 {
		t.Fatalf("deliverCount = %d, want 0", dispatcherClient.deliverCount)
	}
	if len(stream.Snapshot()) != 0 {
		t.Fatalf("non-command blacklist rejection should not write logs: %#v", stream.Snapshot())
	}
}

func TestHandleAdapterEventBlocksCommandWhenNotWhitelistedBeforeBridge(t *testing.T) {
	t.Parallel()

	dispatcherClient := &recordingDispatcherClient{}
	cfg := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngressWithGovernance(
		plugincatalog.New([]plugins.Snapshot{{
			PluginID:          "weather",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name: "weather",
			}},
		}}),
		newStubWhitelistRepo(),
		&stubWhitelistStateRepo{enabled: true},
		nil,
		nil,
		bridge.New(slog.Default(), dispatcherClient),
	)

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-white-1",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        time.Now().Unix(),
		ConversationType: "private",
		ConversationID:   "10001",
		SenderID:         "10001",
		PlainText:        "/weather",
	})

	if dispatcherClient.deliverCount != 0 {
		t.Fatalf("deliverCount = %d, want 0", dispatcherClient.deliverCount)
	}
}

func TestHandleAdapterEventLogsWhitelistedCommandRejection(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	dispatcherClient := &recordingDispatcherClient{}
	cfg := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	application := newTestAppState(cfg, logger)
	application.setTestEventIngressWithGovernance(
		plugincatalog.New([]plugins.Snapshot{{
			PluginID:          "weather",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name: "weather",
			}},
		}}),
		newStubWhitelistRepo(),
		&stubWhitelistStateRepo{enabled: true},
		nil,
		nil,
		bridge.New(logger, dispatcherClient),
	)

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-white-log-1",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        time.Now().Unix(),
		ConversationType: "private",
		ConversationID:   "10001",
		SenderID:         "10001",
		MessageID:        "30001",
		PlainText:        "/weather",
	})

	if dispatcherClient.deliverCount != 0 {
		t.Fatalf("deliverCount = %d, want 0", dispatcherClient.deliverCount)
	}

	summary := waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Message == "插件 weather 的命令 weather 被权限策略拒绝：发送者不在白名单中"
	})
	if summary.Level != "warn" {
		t.Fatalf("unexpected log level: got %q want warn", summary.Level)
	}
	if summary.Source != "bridge" || summary.Protocol != logging.ProtocolOneBot11 {
		t.Fatalf("unexpected log source/protocol: %+v", summary)
	}
	if summary.PluginID != "weather" {
		t.Fatalf("unexpected plugin_id: got %q want weather", summary.PluginID)
	}
	if summary.Details["command_name"] != "weather" || summary.Details["policy_stage"] != "whitelist" {
		t.Fatalf("unexpected whitelist log details: %#v", summary.Details)
	}
	if summary.Details["error_code"] != "permission.not_whitelisted" || summary.Details["reason"] != "发送者不在白名单中" {
		t.Fatalf("unexpected whitelist log details: %#v", summary.Details)
	}
	if !reflect.DeepEqual(summary.Details["matched_plugin_ids"], []any{"weather"}) {
		t.Fatalf("unexpected matched_plugin_ids: %#v", summary.Details["matched_plugin_ids"])
	}
}

func TestHandleAdapterEventLogsBlacklistedCommandRejection(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	repo := newStubBlacklistRepo()
	repo.block("user", "bad-user")
	dispatcherClient := &recordingDispatcherClient{}
	cfg := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	application := newTestAppState(cfg, logger)
	application.setTestEventIngress(
		plugincatalog.New([]plugins.Snapshot{{
			PluginID:          "ops.tools",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name: "ops",
			}},
		}}),
		repo,
		nil,
		bridge.New(logger, dispatcherClient),
	)

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-blacklist-log-1",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        time.Now().Unix(),
		ConversationType: "private",
		ConversationID:   "10001",
		SenderID:         "bad-user",
		MessageID:        "30002",
		PlainText:        "/ops",
	})

	summary := waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Message == "插件 ops.tools 的命令 ops 被权限策略拒绝：用户在黑名单中"
	})
	if summary.Level != "warn" || summary.PluginID != "ops.tools" {
		t.Fatalf("unexpected blacklist summary: %+v", summary)
	}
	if summary.Details["policy_stage"] != "blacklist" || summary.Details["error_code"] != "permission.blacklisted" {
		t.Fatalf("unexpected blacklist details: %#v", summary.Details)
	}
}

func TestHandleAdapterEventUsesMostStrictMatchingCommandPermission(t *testing.T) {
	t.Parallel()

	dispatcherClient := &recordingDispatcherClient{}
	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "weather",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name:       "ops",
				Permission: "everyone",
			}},
		},
		{
			PluginID:          "admin",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name:       "ops",
				Permission: "group_admin",
			}},
		},
	}), nil, nil, bridge.New(slog.Default(), dispatcherClient))

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-ops",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/ops",
		MessageID:        "30001",
	})

	if dispatcherClient.deliverCount != 0 {
		t.Fatalf("deliverCount = %d, want 0", dispatcherClient.deliverCount)
	}
}

func TestHandleAdapterEventLogsPermissionDeniedCommandRejection(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	dispatcherClient := &recordingDispatcherClient{}
	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	application := newTestAppState(cfg, logger)
	application.setTestEventIngress(
		plugincatalog.New([]plugins.Snapshot{{
			PluginID:          "admin",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name:       "ops",
				Permission: "group_admin",
			}},
		}}),
		nil,
		nil,
		bridge.New(logger, dispatcherClient),
	)

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-permission-log-1",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		MessageID:        "30003",
		PlainText:        "/ops",
	})

	summary := waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Message == "插件 admin 的命令 ops 被权限策略拒绝：权限等级不足"
	})
	if summary.Details["policy_stage"] != "permission" || summary.Details["error_code"] != "permission.denied" {
		t.Fatalf("unexpected permission details: %#v", summary.Details)
	}
}

func TestHandleAdapterEventLogsConflictingCommandRejectionWithoutPluginID(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	dispatcherClient := &recordingDispatcherClient{}
	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	application := newTestAppState(cfg, logger)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "weather",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name:       "ops",
				Permission: "everyone",
			}},
		},
		{
			PluginID:          "admin",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name:       "ops",
				Permission: "group_admin",
			}},
		},
	}), nil, nil, bridge.New(logger, dispatcherClient))

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-conflict-log-1",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		MessageID:        "30004",
		PlainText:        "/ops",
	})

	summary := waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Message == "命令 ops 被权限策略拒绝：权限等级不足"
	})
	if summary.PluginID != "" {
		t.Fatalf("expected empty plugin_id for conflicting command, got %q", summary.PluginID)
	}
	if !sameStringItems(summary.Details["matched_plugin_ids"], []string{"weather", "admin"}) {
		t.Fatalf("unexpected matched_plugin_ids: %#v", summary.Details["matched_plugin_ids"])
	}
}

func TestApplyChatPolicySendsCooldownReplyForGroupCommand(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
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
	event := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
		EventID:          "evt-weather",
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
	if _, allowed := application.applyChatPolicy(context.Background(), event); allowed {
		t.Fatal("second command should be rate limited")
	}
	if sender.replyCount != 1 {
		t.Fatalf("replyCount = %d, want 1", sender.replyCount)
	}
	if sender.lastReplyText != chatpolicy.CooldownReplyText {
		t.Fatalf("reply text = %q, want %q", sender.lastReplyText, chatpolicy.CooldownReplyText)
	}
}
