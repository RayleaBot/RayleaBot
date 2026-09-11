package chatpolicy_test

import (
	"context"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/bot/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestCommandInfoForEventUsesDefaultLevelForOmittedPermission(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "group_admin"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	testConfig := cfg
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "weather-admin",
		}},
	}})
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	info := ingress.CommandInfoForEvent(ingress.EnrichCommandEvent(chatevent.NormalizedEvent{
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
	testConfig := config.Config{}
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.BlacklistRepo = repo
	deps.Bridge = bridge.New(slog.Default(), dispatcherClient)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

	logger, stream := newIngressTestLogger(t)
	repo := newStubBlacklistRepo()
	repo.block("user", "bad-user")
	dispatcherClient := &recordingDispatcherClient{}
	testConfig := config.Config{}
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = logger
	deps.BlacklistRepo = repo
	deps.Bridge = bridge.New(logger, dispatcherClient)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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
	testConfig := cfg
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "weather",
		}},
	}})
	deps.WhitelistRepo = newStubWhitelistRepo()
	deps.WhitelistState = &stubWhitelistStateRepo{enabled: true}
	deps.Bridge = bridge.New(slog.Default(), dispatcherClient)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

	logger, stream := newIngressTestLogger(t)
	dispatcherClient := &recordingDispatcherClient{}
	cfg := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	testConfig := cfg
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = logger
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "weather",
		}},
	}})
	deps.WhitelistRepo = newStubWhitelistRepo()
	deps.WhitelistState = &stubWhitelistStateRepo{enabled: true}
	deps.Bridge = bridge.New(logger, dispatcherClient)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

	summary := waitForIngressLog(t, stream, func(summary logging.Summary) bool {
		return summary.PluginID == "weather" && summary.Details["error_code"] == "permission.not_whitelisted"
	})
	if summary.Level != "warn" {
		t.Fatalf("unexpected log level: got %q want warn", summary.Level)
	}
	if summary.Source != "bridge.onebot11" || summary.Protocol != logging.ProtocolOneBot11 {
		t.Fatalf("unexpected log source/protocol: %+v", summary)
	}
	if summary.PluginID != "weather" {
		t.Fatalf("unexpected plugin_id: got %q want weather", summary.PluginID)
	}
	if summary.Details["command_name"] != "weather" || summary.Details["policy_stage"] != "whitelist" {
		t.Fatalf("unexpected whitelist log details: %#v", summary.Details)
	}
	if summary.Details["error_code"] != "permission.not_whitelisted" || summary.Details["reason"] == nil || summary.Details["reason"] == "" {
		t.Fatalf("unexpected whitelist log details: %#v", summary.Details)
	}
	if !reflect.DeepEqual(summary.Details["matched_plugin_ids"], []any{"weather"}) {
		t.Fatalf("unexpected matched_plugin_ids: %#v", summary.Details["matched_plugin_ids"])
	}
}

func TestHandleAdapterEventLogsBlacklistedCommandRejection(t *testing.T) {
	t.Parallel()

	logger, stream := newIngressTestLogger(t)
	repo := newStubBlacklistRepo()
	repo.block("user", "bad-user")
	dispatcherClient := &recordingDispatcherClient{}
	cfg := config.Config{
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	testConfig := cfg
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = logger
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "ops.tools",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name: "ops",
		}},
	}})
	deps.BlacklistRepo = repo
	deps.Bridge = bridge.New(logger, dispatcherClient)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

	summary := waitForIngressLog(t, stream, func(summary logging.Summary) bool {
		return summary.PluginID == "ops.tools" && summary.Details["policy_stage"] == "blacklist"
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
	testConfig := cfg
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{
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
	})
	deps.Bridge = bridge.New(slog.Default(), dispatcherClient)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

	logger, stream := newIngressTestLogger(t)
	dispatcherClient := &recordingDispatcherClient{}
	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	testConfig := cfg
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = logger
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "admin",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "ops",
			Permission: "group_admin",
		}},
	}})
	deps.Bridge = bridge.New(logger, dispatcherClient)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

	summary := waitForIngressLog(t, stream, func(summary logging.Summary) bool {
		return summary.PluginID == "admin" && summary.Details["error_code"] == "permission.denied"
	})
	if summary.Details["policy_stage"] != "permission" || summary.Details["error_code"] != "permission.denied" {
		t.Fatalf("unexpected permission details: %#v", summary.Details)
	}
}

func TestHandleAdapterEventLogsConflictingCommandRejectionWithoutPluginID(t *testing.T) {
	t.Parallel()

	logger, stream := newIngressTestLogger(t)
	dispatcherClient := &recordingDispatcherClient{}
	cfg := config.Config{
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	testConfig := cfg
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = logger
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{
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
	})
	deps.Bridge = bridge.New(logger, dispatcherClient)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

	summary := waitForIngressLog(t, stream, func(summary logging.Summary) bool {
		return summary.Details["command_name"] == "ops" && summary.Details["error_code"] == "permission.denied"
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
	testConfig := cfg
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "weather",
			Permission: "everyone",
		}},
	}})
	deps.OutboundSender = sender
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)
	event := chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

	if _, allowed := ingress.ApplyChatPolicy(context.Background(), event); !allowed {
		t.Fatal("first command should be allowed")
	}
	if _, allowed := ingress.ApplyChatPolicy(context.Background(), event); allowed {
		t.Fatal("second command should be rate limited")
	}
	if sender.replyCount != 1 {
		t.Fatalf("replyCount = %d, want 1", sender.replyCount)
	}
	if sender.lastReplyText != chatpolicy.CooldownReplyText {
		t.Fatalf("reply text = %q, want %q", sender.lastReplyText, chatpolicy.CooldownReplyText)
	}
}
