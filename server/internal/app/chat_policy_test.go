package app

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/runtime"
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
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "weather-admin",
		}},
	}}), nil, nil, nil)

	info := application.commandInfoForEvent(application.enrichCommandEvent(adapter.NormalizedEvent{
		PlainText: "/weather-admin",
	}))
	if info == nil {
		t.Fatal("commandInfoForEvent returned nil")
	}
	if info.Permission != "group_admin" {
		t.Fatalf("permission = %q, want group_admin", info.Permission)
	}
}

func TestResolveChatPolicyConfigPrefersCanonicalFields(t *testing.T) {
	t.Parallel()

	settings := resolveChatPolicyConfig(config.Config{
		Admin:      config.AdminConfig{SuperAdmins: []string{"canonical-admin"}},
		Permission: config.PermissionConfig{DefaultLevel: "group_admin"},
		User: config.UserConfig{
			CommandRateLimit: "2/1h",
			CooldownReply:    false,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "3/1h",
		},
		Auth: config.AuthConfig{
			SuperAdmins:  []string{"legacy-admin"},
			DefaultLevel: "super_admin",
		},
		Cooldown: &config.CooldownConfig{
			UserCommandRateLimit:  "9/1h",
			GroupCommandRateLimit: "8/1h",
			CooldownReply:         true,
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

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
		plugins.NewCatalog([]plugins.Snapshot{{
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

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
		plugins.NewCatalog([]plugins.Snapshot{{
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

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
		return summary.Message == "plugin weather command weather rejected by command policy: sender is not whitelisted"
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
	if summary.Details["error_code"] != "permission.not_whitelisted" || summary.Details["reason"] != "actor is not whitelisted" {
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
		plugins.NewCatalog([]plugins.Snapshot{{
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

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
		return summary.Message == "plugin ops.tools command ops rejected by command policy: user is blacklisted"
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
		Auth: config.AuthConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{
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

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
		Auth: config.AuthConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	application := newTestAppState(cfg, logger)
	application.setTestEventIngress(
		plugins.NewCatalog([]plugins.Snapshot{{
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

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
		return summary.Message == "plugin admin command ops rejected by command policy: insufficient permission level"
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
		Auth: config.AuthConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	application := newTestAppState(cfg, logger)
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{
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

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
		return summary.Message == "command ops rejected by command policy: insufficient permission level"
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
		Cooldown: &config.CooldownConfig{
			UserCommandRateLimit:  "1/1h",
			GroupCommandRateLimit: "5/1h",
			CooldownReply:         true,
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
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
	event := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
	if sender.lastReplyText != cooldownReplyText {
		t.Fatalf("reply text = %q, want %q", sender.lastReplyText, cooldownReplyText)
	}
}

func TestApplyChatPolicyAppliesTargetLimitToCooldownReply(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	sender := &recordingOutboundSender{}
	limiter := &recordingAppOutboundLimiter{
		err: &adapter.Error{Code: "platform.rate_limited", Message: "outbound message rate limit exceeded"},
	}
	cfg := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		Cooldown: &config.CooldownConfig{
			UserCommandRateLimit:  "1/1h",
			GroupCommandRateLimit: "5/1h",
			CooldownReply:         true,
		},
	}
	application := newTestAppState(cfg, logger)
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
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
	application.eventIngress.outboundLimiter = limiter

	event := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
		Cooldown: &config.CooldownConfig{
			UserCommandRateLimit:  "1/1h",
			GroupCommandRateLimit: "5/1h",
			CooldownReply:         true,
		},
	}
	application := newTestAppState(cfg, nil)
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
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
	application.eventIngress.outboundLimiter = limiter

	event := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "help",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "help",
		}},
	}}), nil, nil, nil)
	event := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "weather",
		}},
	}}), nil, nil, nil)
	event := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "weather",
		}},
	}}), nil, nil, nil)
	firstEvent := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "weather",
		}},
	}}), nil, sender, nil)
	event := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "ops",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "ops",
		}},
	}}), nil, nil, nil)

	memberEvent := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
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
	}, nil)
	application.renderer = newRenderServiceForRepo(t, repoRoot, renderRoot, runner)
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
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

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-builtin-menu",
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

func TestHandleAdapterEventUsesIndependentBuiltinMenuPrefix(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	dispatcher := &recordingDispatcherClient{}
	application := newTestAppState(config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"help", "帮助"},
			Prefixes: []string{"#"},
		}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}, nil)
	application.renderer = newRenderService(t, t.TempDir())
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "fortune",
		Name:              "运势",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "fortune",
			Permission: "everyone",
		}},
	}}), nil, sender, bridge.New(slog.Default(), dispatcher))

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-builtin-menu-prefix",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        time.Now().Unix(),
		ConversationType: "private",
		ConversationID:   "10002",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "#help fortune",
		MessageID:        "30002",
	})

	if sender.messageCount != 1 || !strings.HasPrefix(sender.lastMessageImage, "file://") {
		t.Fatalf("unexpected builtin menu send: count=%d image=%q", sender.messageCount, sender.lastMessageImage)
	}
	if dispatcher.deliverCount != 0 {
		t.Fatalf("builtin menu dispatched to plugins %d times", dispatcher.deliverCount)
	}
}

func TestApplyChatPolicyDoesNotTreatPluginCommandAsBuiltinWhenMenuPrefixDiffers(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"help"},
			Prefixes: []string{"#"},
		}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}, nil)
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "admin-help",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "help",
			Permission: "super_admin",
		}},
	}}), nil, nil, bridge.New(slog.Default(), &recordingDispatcherClient{}))

	_, allowed := application.applyChatPolicy(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-plugin-help-policy",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/help",
		MessageID:        "30006",
	})
	if allowed {
		t.Fatal("plugin /help command should keep plugin permission when builtin menu prefix is different")
	}
}

func TestBuiltinRootMenuDataUsesBuiltinMenuPrefixesAndTriggerExamples(t *testing.T) {
	t.Parallel()

	application := newTestAppState(config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"帮助"},
			Prefixes: []string{"#"},
		}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}, nil)
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "fortune",
		Name:              "运势",
		Description:       "今日运势",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "fortune",
			Usage:      "/fortune",
			Permission: "everyone",
		}},
	}}), nil, nil, bridge.New(slog.Default(), &recordingDispatcherClient{}))

	data := application.eventIngress.buildBuiltinMenuData(adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		ConversationType: "private",
		ConversationID:   "10002",
		SenderID:         "10002",
		ActorRole:        "member",
	}, "")
	if got := data["command_prefixes"]; !reflect.DeepEqual(got, []string{"#"}) {
		t.Fatalf("command_prefixes = %#v, want [#]", got)
	}
	if got := data["trigger_examples"]; !reflect.DeepEqual(got, []string{"#帮助 运势"}) {
		t.Fatalf("trigger_examples = %#v, want [#帮助 运势]", got)
	}
	items, ok := data["items"].([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected root menu items: %#v", data["items"])
	}
	if _, ok := items[0]["usage"]; ok {
		t.Fatalf("root menu item should not include usage: %#v", items[0])
	}
}

func TestBuiltinRootMenuDataFallsBackToCommandPrefixes(t *testing.T) {
	t.Parallel()

	data := builtinRootMenuData([]map[string]any{{
		"id":          "echo",
		"name":        "Echo",
		"description": "复读消息",
	}}, config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"X", "C"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"help", "帮助"},
		}},
	})

	if got := data["command_prefixes"]; !reflect.DeepEqual(got, []string{"X", "C"}) {
		t.Fatalf("command_prefixes = %#v, want [X C]", got)
	}
	if got := data["trigger_examples"]; !reflect.DeepEqual(got, []string{"Xhelp Echo", "CEcho帮助"}) {
		t.Fatalf("trigger_examples = %#v, want [Xhelp Echo CEcho帮助]", got)
	}
}

func TestBuiltinPluginMenuDataUsesMenuPrefixesWithoutTriggerExamples(t *testing.T) {
	t.Parallel()

	data := builtinPluginMenuData(map[string]any{
		"id":          "subscription-hub",
		"name":        "订阅中心",
		"description": "订阅平台内容并推送更新",
		"commands": buildBuiltinCommands([]plugins.CommandView{{
			Name:        "全部b站订阅列表",
			Description: "查看所有群聊和私聊的 Bilibili 订阅列表",
			Usage:       "/全部b站订阅列表",
			Permission:  "super_admin",
		}}, config.Config{Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Prefixes: []string{"#", "*"}}}}),
	}, config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Prefixes: []string{"#", "*"},
		}},
	})

	if _, ok := data["trigger_examples"]; ok {
		t.Fatalf("plugin menu should not include trigger_examples: %#v", data)
	}
	if got := data["command_prefixes"]; !reflect.DeepEqual(got, []string{"#", "*"}) {
		t.Fatalf("plugin command_prefixes = %#v, want [# *]", got)
	}
	groups, ok := data["groups"].([]map[string]any)
	if !ok || len(groups) == 0 {
		t.Fatalf("unexpected plugin menu groups: %#v", data["groups"])
	}
	items, ok := groups[0]["items"].([]map[string]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected command items: %#v", groups[0]["items"])
	}
	if got := items[0]["command_prefixes"]; !reflect.DeepEqual(got, []string{"#", "*"}) {
		t.Fatalf("command item prefixes = %#v, want [# *]", got)
	}
	if _, ok := items[0]["usage"]; ok {
		t.Fatalf("command item should not include usage: %#v", items[0])
	}
}

func TestHandleAdapterEventMatchesBuiltinPluginSuffixHelp(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	application := newTestAppState(config.Config{
		Command:    &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin:    config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Commands: []string{"help", "帮助"}}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}, nil)
	application.renderer = newRenderService(t, t.TempDir())
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "fortune",
		Name:              "运势",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "fortune",
			Aliases:    []string{"运势"},
			Permission: "everyone",
		}},
	}}), nil, sender, bridge.New(slog.Default(), &recordingDispatcherClient{}))

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-builtin-menu-suffix",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/运势帮助",
		MessageID:        "30003",
	})

	if sender.replyCount != 1 || !strings.HasPrefix(sender.lastReplyImage, "file://") {
		t.Fatalf("unexpected suffix menu reply: count=%d image=%q", sender.replyCount, sender.lastReplyImage)
	}
}

func TestHandleAdapterEventSkipsMissingBuiltinPluginMenuTarget(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	dispatcher := &recordingDispatcherClient{}
	application := newTestAppState(config.Config{
		Command:    &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin:    config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Commands: []string{"help", "帮助"}}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}, nil)
	application.renderer = newRenderService(t, t.TempDir())
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "fortune",
		Name:              "运势",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "fortune",
			Aliases:    []string{"运势"},
			Permission: "everyone",
		}},
	}}), nil, sender, bridge.New(slog.Default(), dispatcher))

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-missing-builtin-menu-target",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/表情帮助",
		MessageID:        "30008",
	})

	if sender.replyCount != 0 || sender.messageCount != 0 {
		t.Fatalf("missing builtin menu target sent outbound message: replies=%d messages=%d text=%q image=%q", sender.replyCount, sender.messageCount, sender.lastReplyText, sender.lastReplyImage)
	}
	if dispatcher.deliverCount != 0 {
		t.Fatalf("missing builtin menu target dispatched to plugins %d times", dispatcher.deliverCount)
	}
}

func TestHandleAdapterEventDoesNotTreatExactPluginCommandAsBuiltinSuffixMenu(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	dispatcher := &recordingDispatcherClient{}
	application := newTestAppState(config.Config{
		Command:    &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin:    config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Commands: []string{"help", "帮助"}}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}, nil)
	application.renderer = newRenderService(t, t.TempDir())
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "custom-help",
		Name:              "Custom Help",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "myhelp",
			Permission: "everyone",
		}},
	}}), nil, sender, bridge.New(slog.Default(), dispatcher))

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-plugin-command-help-suffix",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/myhelp",
		MessageID:        "30007",
	})

	if sender.replyCount != 0 || sender.messageCount != 0 {
		t.Fatalf("plugin command was handled as builtin menu: replies=%d messages=%d", sender.replyCount, sender.messageCount)
	}
	if dispatcher.deliverCount != 1 {
		t.Fatalf("plugin command dispatch count = %d, want 1", dispatcher.deliverCount)
	}
}

func TestHandleAdapterEventBlocksBuiltinMenuWhenBlacklistApplies(t *testing.T) {
	t.Parallel()

	repo := newStubBlacklistRepo()
	repo.block("user", "blocked-user")
	sender := &recordingOutboundSender{}
	dispatcher := &recordingDispatcherClient{}
	application := newTestAppState(config.Config{
		Command:    &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin:    config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Commands: []string{"help"}}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}, nil)
	application.renderer = newRenderService(t, t.TempDir())
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "weather",
			Permission: "everyone",
		}},
	}}), repo, sender, bridge.New(slog.Default(), dispatcher))

	application.handleAdapterEvent(context.Background(), adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-builtin-menu-blacklist",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        time.Now().Unix(),
		ConversationType: "private",
		ConversationID:   "blocked-user",
		SenderID:         "blocked-user",
		ActorRole:        "member",
		PlainText:        "/help",
		MessageID:        "30004",
	})

	if sender.replyCount != 0 || sender.messageCount != 0 {
		t.Fatalf("blocked builtin menu should not send response: replies=%d messages=%d", sender.replyCount, sender.messageCount)
	}
	if dispatcher.deliverCount != 0 {
		t.Fatalf("blocked builtin menu dispatched to plugins %d times", dispatcher.deliverCount)
	}
}

func TestHandleAdapterEventBlocksBuiltinMenuWhenCooldownApplies(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	dispatcher := &recordingDispatcherClient{}
	application := newTestAppState(config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"help"},
		}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		User: config.UserConfig{
			CommandRateLimit: "1/1h",
			CooldownReply:    false,
		},
		Group: config.GroupConfig{CommandRateLimit: "10/1h"},
	}, nil)
	application.renderer = newRenderService(t, t.TempDir())
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "weather",
			Permission: "everyone",
		}},
	}}), nil, sender, bridge.New(slog.Default(), dispatcher))

	event := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-builtin-menu-cooldown",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.private",
		Timestamp:        time.Now().Unix(),
		ConversationType: "private",
		ConversationID:   "10002",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "/help",
		MessageID:        "30005",
	}
	application.handleAdapterEvent(context.Background(), event)
	application.handleAdapterEvent(context.Background(), event)

	if sender.messageCount != 1 {
		t.Fatalf("builtin menu should send once before cooldown, messages=%d", sender.messageCount)
	}
	if dispatcher.deliverCount != 0 {
		t.Fatalf("builtin menu dispatched to plugins %d times", dispatcher.deliverCount)
	}
}

func TestApplyChatPolicyLogsCooldownReplySuccess(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	sender := &recordingOutboundSender{}
	cfg := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		Cooldown: &config.CooldownConfig{
			UserCommandRateLimit:  "1/1h",
			GroupCommandRateLimit: "5/1h",
			CooldownReply:         true,
		},
	}
	application := newTestAppState(cfg, logger)
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
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

	event := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-weather-log-success",
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

	summary := waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Message == "plugin weather command weather rejected by command policy: user command rate limited"
	})
	if summary.Level != "warn" || summary.Source != "bridge" {
		t.Fatalf("unexpected cooldown rejection summary: %+v", summary)
	}
	if summary.Details["policy_stage"] != "cooldown" || summary.Details["error_code"] != "platform.user_rate_limited" {
		t.Fatalf("unexpected cooldown rejection details: %#v", summary.Details)
	}

	summary = waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Message == "系统 -> [测试群(20001)]：命令触发冷却，请稍后再试。"
	})
	if summary.Source != "adapter.onebot11" {
		t.Fatalf("unexpected log source: got %q want adapter.onebot11", summary.Source)
	}
	if summary.Level != "info" {
		t.Fatalf("unexpected log level: got %q want info", summary.Level)
	}
	if summary.Details["action_kind"] != "message.reply" || summary.Details["delivery_kind"] != "message.reply" {
		t.Fatalf("unexpected action details: %#v", summary.Details)
	}
	if summary.Details["target_type"] != "group" || summary.Details["target_id"] != "20001" {
		t.Fatalf("unexpected target details: %#v", summary.Details)
	}
	if summary.Details["message_id"] != "msg-2" {
		t.Fatalf("unexpected message_id detail: %#v", summary.Details["message_id"])
	}
}

func TestApplyChatPolicyLogsCooldownReplyFailure(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	sender := &recordingOutboundSender{
		replyErr: &adapter.Error{Code: "adapter.send_failed", Message: "cooldown reply blocked"},
	}
	cfg := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		Cooldown: &config.CooldownConfig{
			UserCommandRateLimit:  "1/1h",
			GroupCommandRateLimit: "5/1h",
			CooldownReply:         true,
		},
	}
	application := newTestAppState(cfg, logger)
	application.setTestEventIngress(plugins.NewCatalog([]plugins.Snapshot{{
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

	event := adapter.NormalizedEvent{
		Kind:             adapter.EventKindMessage,
		EventID:          "evt-weather-log-failure",
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

	summary := waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Message == "plugin weather command weather rejected by command policy: user command rate limited"
	})
	if summary.Level != "warn" {
		t.Fatalf("unexpected cooldown rejection level: got %q want warn", summary.Level)
	}
	if summary.Details["policy_stage"] != "cooldown" || summary.Details["error_code"] != "platform.user_rate_limited" {
		t.Fatalf("unexpected cooldown rejection details: %#v", summary.Details)
	}

	summary = waitForAppLog(t, stream, func(summary logging.Summary) bool {
		return summary.Message == "系统 -> [测试群(20001)] 发送失败：命令触发冷却，请稍后再试。"
	})
	if summary.Level != "warn" {
		t.Fatalf("unexpected log level: got %q want warn", summary.Level)
	}
	if summary.Details["error_code"] != "adapter.send_failed" {
		t.Fatalf("unexpected error_code detail: %#v", summary.Details["error_code"])
	}
	if summary.Details["reason"] != "cooldown reply blocked" {
		t.Fatalf("unexpected reason detail: %#v", summary.Details["reason"])
	}
}

func TestApplyHotReloadableFieldsReloadsCommandPolicy(t *testing.T) {
	t.Parallel()

	repo := newStubBlacklistRepo()
	cfg := config.Config{
		Admin: config.AdminConfig{
			SuperAdmins: []string{"1"},
		},
		Permission: config.PermissionConfig{
			DefaultLevel: "everyone",
		},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
		User: config.UserConfig{
			CommandRateLimit: "5/1h",
			CooldownReply:    false,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "5/1h",
		},
		Storage: config.StorageConfig{
			KVValueMaxBytes: 1024,
			KVTotalLimitMB:  8,
			FileMaxBytes:    2048,
			PluginWorkDirMB: 32,
		},
		HTTP: config.HTTPConfig{
			TimeoutSeconds:    10,
			MaxRetries:        0,
			AllowPrivateHosts: []string{},
		},
		Logging: config.LoggingConfig{Level: "info"},
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "1/1h",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
		},
	}
	app := newTestAppState(cfg, nil)
	app.setTestEventIngress(nil, repo, nil, nil)
	app.outboundLimiter = outbound.NewMessageRateLimiter(cfg)
	if err := app.outboundLimiter.Wait(context.Background(), outbound.MessageLimitRequest{
		PluginID:   "weather",
		TargetType: "group",
		TargetID:   "20001",
	}); err != nil {
		t.Fatalf("prime outbound limiter: %v", err)
	}

	restartRequired := applyHotReloadableFields(app, config.Config{
		Admin: config.AdminConfig{
			SuperAdmins: []string{"42"},
		},
		Permission: config.PermissionConfig{
			DefaultLevel: "group_admin",
		},
		Command: &config.CommandConfig{
			Prefixes: []string{"!"},
		},
		User: config.UserConfig{
			CommandRateLimit: "1/1h",
			CooldownReply:    true,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "2/1h",
		},
		Storage: config.StorageConfig{
			KVValueMaxBytes: 4096,
			KVTotalLimitMB:  16,
			FileMaxBytes:    8192,
			PluginWorkDirMB: 64,
		},
		HTTP: config.HTTPConfig{
			TimeoutSeconds:    15,
			MaxRetries:        2,
			AllowPrivateHosts: []string{"127.0.0.1"},
		},
		Logging: config.LoggingConfig{Level: "info"},
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "2/1h",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
		},
	})
	if restartRequired {
		t.Fatal("restartRequired = true, want false for hot-reloadable fields")
	}
	if !app.eventIngress.commandParser.Parse("!ping").IsCommand {
		t.Fatal("new command prefix was not applied")
	}
	if app.eventIngress.commandParser.Parse("/ping").IsCommand {
		t.Fatal("old command prefix should no longer be active")
	}
	if verdict := app.eventIngress.permissionChecker.Check(context.Background(), "42", "member", "", &permission.CommandInfo{Permission: "super_admin"}); !verdict.Allowed {
		t.Fatalf("new super admin should bypass command checks: %#v", verdict)
	}
	if verdict := app.eventIngress.permissionChecker.Check(context.Background(), "1", "member", "", &permission.CommandInfo{Permission: "super_admin"}); verdict.Allowed {
		t.Fatalf("old super admin should no longer bypass command checks: %#v", verdict)
	}
	if app.state.Config.Storage.FileMaxBytes != 8192 || app.state.Config.Storage.PluginWorkDirMB != 64 {
		t.Fatalf("storage config was not hot reloaded: %+v", app.state.Config.Storage)
	}
	if app.state.Config.HTTP.TimeoutSeconds != 15 || app.state.Config.HTTP.MaxRetries != 2 {
		t.Fatalf("http config was not hot reloaded: %+v", app.state.Config.HTTP)
	}
	if len(app.state.Config.HTTP.AllowPrivateHosts) != 1 || app.state.Config.HTTP.AllowPrivateHosts[0] != "127.0.0.1" {
		t.Fatalf("http allow_private_hosts was not hot reloaded: %+v", app.state.Config.HTTP.AllowPrivateHosts)
	}
	if err := app.outboundLimiter.Wait(context.Background(), outbound.MessageLimitRequest{
		PluginID:   "weather",
		TargetType: "group",
		TargetID:   "20002",
	}); err != nil {
		t.Fatalf("new outbound message limit was not applied: %v", err)
	}
}

type recordingDispatcherClient struct {
	deliverCount int
}

func (r *recordingDispatcherClient) HasDeliverablePlugins() bool {
	return true
}

func (r *recordingDispatcherClient) Dispatch(_ context.Context, _ runtime.Event, _ string) []dispatch.DeliveryResult {
	r.deliverCount++
	return []dispatch.DeliveryResult{{
		PluginID: "test",
		Outcome:  dispatch.OutcomeDelivered,
	}}
}

type recordingOutboundSender struct {
	replyCount       int
	lastReplyText    string
	lastReplyImage   string
	messageCount     int
	lastMessageText  string
	lastMessageImage string
	replyErr         error
	messageErr       error
}

func (s *recordingOutboundSender) SendMessage(_ context.Context, action adapter.OutboundMessageSend) (adapter.SendMessageResult, error) {
	s.messageCount++
	s.lastMessageText = firstTextSegment(action.Segments)
	s.lastMessageImage = firstImageSegment(action.Segments)
	return adapter.SendMessageResult{MessageID: "msg-1"}, s.messageErr
}

func (s *recordingOutboundSender) SendReply(_ context.Context, action adapter.OutboundMessageReply) (adapter.SendMessageResult, error) {
	s.replyCount++
	s.lastReplyText = firstTextSegment(action.Segments)
	s.lastReplyImage = firstImageSegment(action.Segments)
	return adapter.SendMessageResult{MessageID: "msg-2"}, s.replyErr
}

type recordingAppOutboundLimiter struct {
	requests []outbound.MessageLimitRequest
	err      error
}

func (l *recordingAppOutboundLimiter) Wait(_ context.Context, request outbound.MessageLimitRequest) error {
	l.requests = append(l.requests, request)
	return l.err
}

func (l *recordingAppOutboundLimiter) ApplyConfig(config.Config) {}

func (l *recordingAppOutboundLimiter) lastRequest() outbound.MessageLimitRequest {
	if len(l.requests) == 0 {
		return outbound.MessageLimitRequest{}
	}
	return l.requests[len(l.requests)-1]
}

type contextAwareOutboundLimiter struct {
	ctxErr error
}

func (l *contextAwareOutboundLimiter) Wait(ctx context.Context, _ outbound.MessageLimitRequest) error {
	l.ctxErr = ctx.Err()
	if l.ctxErr != nil {
		return &adapter.Error{Code: "platform.rate_limited", Message: "outbound message rate limit exceeded"}
	}
	return nil
}

func firstTextSegment(segments []adapter.OutboundMessageSegment) string {
	for _, segment := range segments {
		if segment.Type != "text" {
			continue
		}
		if text, ok := segment.Data["text"].(string); ok {
			return text
		}
	}
	return ""
}

func firstImageSegment(segments []adapter.OutboundMessageSegment) string {
	for _, segment := range segments {
		if segment.Type != "image" {
			continue
		}
		if file, ok := segment.Data["file"].(string); ok {
			return file
		}
	}
	return ""
}

func newAppTestLogger() (*slog.Logger, *logging.Stream) {
	stream := logging.NewStream(16)
	writer := logging.NewSummaryWriter(io.Discard, stream, nil)
	logger := slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attr slog.Attr) slog.Attr {
			switch attr.Key {
			case slog.TimeKey:
				attr.Key = "ts"
			case slog.MessageKey:
				attr.Key = "msg"
			}
			return attr
		},
	}))
	return logger, stream
}

func waitForAppLog(t *testing.T, stream *logging.Stream, match func(logging.Summary) bool) logging.Summary {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		for _, summary := range stream.Snapshot() {
			if match(summary) {
				return summary
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for application log")
	return logging.Summary{}
}

type stubBlacklistRepo struct {
	blocked map[string]map[string]bool
}

func newStubBlacklistRepo() *stubBlacklistRepo {
	return &stubBlacklistRepo{blocked: make(map[string]map[string]bool)}
}

func (s *stubBlacklistRepo) block(entryType, targetID string) {
	if s.blocked[entryType] == nil {
		s.blocked[entryType] = make(map[string]bool)
	}
	s.blocked[entryType][targetID] = true
}

func (s *stubBlacklistRepo) IsBlacklisted(_ context.Context, entryType, targetID string) (bool, error) {
	if entries, ok := s.blocked[entryType]; ok {
		return entries[targetID], nil
	}
	return false, nil
}

func (s *stubBlacklistRepo) Get(_ context.Context, entryType, targetID string) (permission.BlacklistEntry, error) {
	if blocked, _ := s.IsBlacklisted(context.Background(), entryType, targetID); blocked {
		return permission.BlacklistEntry{
			EntryType: entryType,
			TargetID:  targetID,
			Reason:    "blocked",
			CreatedAt: "2026-04-19T00:00:00Z",
		}, nil
	}
	return permission.BlacklistEntry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubBlacklistRepo) Add(context.Context, string, string, string) error {
	return nil
}

func (s *stubBlacklistRepo) Remove(context.Context, string, string) error {
	return nil
}

func (s *stubBlacklistRepo) List(context.Context, string) ([]permission.BlacklistEntry, error) {
	return nil, nil
}

type stubWhitelistRepo struct {
	allowed map[string]map[string]bool
}

func newStubWhitelistRepo() *stubWhitelistRepo {
	return &stubWhitelistRepo{allowed: make(map[string]map[string]bool)}
}

func (s *stubWhitelistRepo) IsWhitelisted(_ context.Context, entryType, targetID string) (bool, error) {
	if entries, ok := s.allowed[entryType]; ok {
		return entries[targetID], nil
	}
	return false, nil
}

func (s *stubWhitelistRepo) Get(_ context.Context, entryType, targetID string) (permission.WhitelistEntry, error) {
	if allowed, _ := s.IsWhitelisted(context.Background(), entryType, targetID); allowed {
		return permission.WhitelistEntry{
			EntryType: entryType,
			TargetID:  targetID,
			Reason:    "allowed",
			CreatedAt: "2026-04-19T00:00:00Z",
		}, nil
	}
	return permission.WhitelistEntry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubWhitelistRepo) Add(context.Context, string, string, string) error {
	return nil
}

func (s *stubWhitelistRepo) Remove(context.Context, string, string) error {
	return nil
}

func (s *stubWhitelistRepo) List(context.Context, string) ([]permission.WhitelistEntry, error) {
	return nil, nil
}

type stubWhitelistStateRepo struct {
	enabled bool
}

func (s *stubWhitelistStateRepo) Enabled(context.Context) (bool, error) {
	return s.enabled, nil
}

func (s *stubWhitelistStateRepo) SetEnabled(_ context.Context, enabled bool) error {
	s.enabled = enabled
	return nil
}

func TestReloadDisablesPluginWhenGrantExpired(t *testing.T) {
	t.Parallel()

	controller, catalog := newLifecycleControllerForGrantTests(t, []plugins.PluginGrant{{
		PluginID:   "weather",
		Capability: "http.request",
		GrantedAt:  time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt:  timePtr(time.Now().UTC().Add(-time.Hour)),
	}})

	_, err := controller.Reload(context.Background(), "weather")
	if err == nil {
		t.Fatal("expected Reload to fail for expired required grant")
	}
	if _, ok := err.(*plugins.PermissionPendingError); !ok {
		t.Fatalf("err = %T, want *plugins.PermissionPendingError", err)
	}

	snapshot, ok := catalog.Get("weather")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.DesiredState != "disabled" {
		t.Fatalf("desired_state = %q, want disabled", snapshot.DesiredState)
	}
}

func TestReloadReturnsPermissionPendingWhenGrantScopeChanged(t *testing.T) {
	t.Parallel()

	controller, catalog := newLifecycleControllerForGrantTests(t, []plugins.PluginGrant{{
		PluginID:   "weather",
		Capability: "http.request",
		GrantedAt:  time.Now().UTC().Add(-2 * time.Hour),
		ScopeJSON:  `{"http_hosts":["legacy.example"]}`,
	}})

	_, err := controller.Reload(context.Background(), "weather")
	if err == nil {
		t.Fatal("expected Reload to fail when grant scope changed")
	}
	pending, ok := err.(*plugins.PermissionPendingError)
	if !ok {
		t.Fatalf("err = %T, want *plugins.PermissionPendingError", err)
	}
	if !pending.ScopeChanged {
		t.Fatalf("ScopeChanged = %v, want true", pending.ScopeChanged)
	}
	if len(pending.MissingCapabilities) != 0 {
		t.Fatalf("MissingCapabilities = %#v, want empty", pending.MissingCapabilities)
	}

	snapshot, ok := catalog.Get("weather")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.DesiredState != "disabled" {
		t.Fatalf("desired_state = %q, want disabled", snapshot.DesiredState)
	}
}

func TestReconcileRuntimeDisablesPluginWhenGrantExpired(t *testing.T) {
	t.Parallel()

	controller, catalog := newLifecycleControllerForGrantTests(t, []plugins.PluginGrant{{
		PluginID:   "weather",
		Capability: "http.request",
		GrantedAt:  time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt:  timePtr(time.Now().UTC().Add(-time.Hour)),
	}})

	controller.reconcileRuntime(context.Background(), "10001")

	snapshot, ok := catalog.Get("weather")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.DesiredState != "disabled" {
		t.Fatalf("desired_state = %q, want disabled", snapshot.DesiredState)
	}
}

func TestStartRuntimeDisablesPluginWhenGrantExpired(t *testing.T) {
	t.Parallel()

	controller, catalog := newLifecycleControllerForGrantTests(t, []plugins.PluginGrant{{
		PluginID:   "weather",
		Capability: "http.request",
		GrantedAt:  time.Now().UTC().Add(-2 * time.Hour),
		ExpiresAt:  timePtr(time.Now().UTC().Add(-time.Hour)),
	}})
	manager := runtime.New(slog.Default(), runtime.Options{})

	if err := controller.startRuntime(context.Background(), "weather", "10001", manager); err != nil {
		t.Fatalf("startRuntime returned error: %v", err)
	}

	snapshot, ok := catalog.Get("weather")
	if !ok {
		t.Fatal("plugin missing from catalog")
	}
	if snapshot.DesiredState != "disabled" {
		t.Fatalf("desired_state = %q, want disabled", snapshot.DesiredState)
	}
}

func newLifecycleControllerForGrantTests(t *testing.T, grants []plugins.PluginGrant) (*pluginLifecycleController, *plugins.Catalog) {
	t.Helper()

	catalog := plugins.NewCatalog([]plugins.Snapshot{{
		PluginID:            "weather",
		Valid:               true,
		RegistrationState:   "installed",
		DesiredState:        "enabled",
		RuntimeState:        "running",
		RequiredPermissions: []string{"http.request"},
	}})
	app := newTestAppState(config.Config{}, slog.Default())
	app.setTestLifecycle(
		catalog,
		nil,
		&stubLifecycleGrantRepository{
			grants: map[string][]plugins.PluginGrant{
				"weather": grants,
			},
		},
		newRuntimeRegistry(slog.Default(), runtime.Options{}),
		dispatch.New(slog.Default(), nil, nil, 16),
		nil,
		nil,
		newPluginWebhookRegistry(),
	)
	return app.pluginLifecycle, catalog
}

type stubLifecycleGrantRepository struct {
	grants map[string][]plugins.PluginGrant
}

func (r *stubLifecycleGrantRepository) LoadGrants(_ context.Context, pluginID string) ([]plugins.PluginGrant, error) {
	now := time.Now().UTC()
	var active []plugins.PluginGrant
	for _, grant := range r.grants[pluginID] {
		if grant.ExpiresAt != nil && !grant.ExpiresAt.After(now) {
			continue
		}
		active = append(active, grant)
	}
	return active, nil
}

func (r *stubLifecycleGrantRepository) LoadAllGrants(_ context.Context) (map[string][]string, error) {
	result := make(map[string][]string)
	for pluginID := range r.grants {
		items, _ := r.LoadGrants(context.Background(), pluginID)
		for _, grant := range items {
			result[pluginID] = append(result[pluginID], grant.Capability)
		}
	}
	return result, nil
}

func (r *stubLifecycleGrantRepository) SaveGrant(context.Context, plugins.PluginGrant) error {
	return nil
}

func (r *stubLifecycleGrantRepository) DeleteGrant(context.Context, string, string) error {
	return nil
}

func (r *stubLifecycleGrantRepository) DeleteAllGrants(context.Context, string) error {
	return nil
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func sameStringItems(actual any, expected []string) bool {
	items, ok := actual.([]any)
	if !ok {
		return false
	}

	got := make([]string, 0, len(items))
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			return false
		}
		got = append(got, value)
	}

	slices.Sort(got)
	expectedCopy := append([]string(nil), expected...)
	slices.Sort(expectedCopy)
	return reflect.DeepEqual(got, expectedCopy)
}
