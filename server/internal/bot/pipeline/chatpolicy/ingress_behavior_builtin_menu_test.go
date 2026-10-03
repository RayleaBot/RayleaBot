package chatpolicy_test

import (
	"context"
	"log/slog"
	"strings"
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
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestHandleAdapterEventUsesIndependentBuiltinMenuPrefix(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	dispatcher := &recordingDispatcherClient{}
	testConfig := config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"help", "帮助"},
			Prefixes: []string{"#"},
		}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	menuRenderer := testutil.NewRenderService(t, t.TempDir())
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{{
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
	}})
	deps.OutboundSender = sender
	deps.Bridge = bridge.New(slog.Default(), dispatcher)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Renderer: menuTestRenderer(menuRenderer), Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

	testConfig := config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"help"},
			Prefixes: []string{"#"},
		}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "admin-help",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Commands: []plugins.Command{{
			Name:       "help",
			Permission: "super_admin",
		}},
	}})
	deps.Bridge = bridge.New(slog.Default(), &recordingDispatcherClient{})
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	_, allowed := ingress.ApplyChatPolicy(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

func TestHandleAdapterEventRepliesWithPluginMenuMatchedByPluginName(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	testConfig := config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"help", "帮助"},
			Prefixes: []string{"#", "*"},
		}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	menuRenderer := testutil.NewRenderService(t, t.TempDir())
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "subscription-hub",
		Name:              "订阅中心",
		Description:       "订阅平台内容并推送更新",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		RuntimeState:      "running",
		Version:           "0.1.0",
		Commands: []plugins.Command{{
			Name:        "订阅状态",
			Description: "查看订阅状态",
			Usage:       "#订阅状态",
			Permission:  "everyone",
		}},
	}})
	deps.OutboundSender = sender
	deps.Bridge = bridge.New(slog.Default(), &recordingDispatcherClient{})
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Renderer: menuTestRenderer(menuRenderer), Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
		EventID:          "evt-builtin-plugin-menu-prefix-group",
		SourceProtocol:   "onebot11",
		SourceAdapter:    "adapter.onebot11",
		EventType:        "message.group",
		Timestamp:        time.Now().Unix(),
		ConversationType: "group",
		ConversationID:   "20001",
		SenderID:         "10002",
		ActorRole:        "member",
		PlainText:        "#help 订阅中心",
		MessageID:        "30004",
	})

	// "#help 订阅中心" names the plugin by its display name rather than by a
	// command, and still opens that plugin's menu page as a reply in the group.
	if sender.replyCount != 1 || !strings.HasPrefix(sender.lastReplyImage, "file://") {
		t.Fatalf("unexpected plugin menu reply: count=%d image=%q", sender.replyCount, sender.lastReplyImage)
	}
}

func TestHandleAdapterEventSkipsMissingBuiltinPluginMenuTarget(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	dispatcher := &recordingDispatcherClient{}
	testConfig := config.Config{
		Command:    &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin:    config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Commands: []string{"help", "帮助"}}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	menuRenderer := testutil.NewRenderService(t, t.TempDir())
	deps.Plugins = plugincatalog.New([]plugins.Snapshot{{
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
	}})
	deps.OutboundSender = sender
	deps.Bridge = bridge.New(slog.Default(), dispatcher)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Renderer: menuTestRenderer(menuRenderer), Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

func TestHandleAdapterEventBlocksBuiltinMenuWhenBlacklistApplies(t *testing.T) {
	t.Parallel()

	repo := newStubBlacklistRepo()
	repo.blockUser("blocked-user")
	sender := &recordingOutboundSender{}
	dispatcher := &recordingDispatcherClient{}
	testConfig := config.Config{
		Command:    &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin:    config.BuiltinConfig{Menu: config.BuiltinMenuConfig{Commands: []string{"help"}}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	menuRenderer := testutil.NewRenderService(t, t.TempDir())
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
	deps.BlacklistRepo = repo
	deps.OutboundSender = sender
	deps.Bridge = bridge.New(slog.Default(), dispatcher)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Renderer: menuTestRenderer(menuRenderer), Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	ingress.HandleAdapterEvent(context.Background(), chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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
	testConfig := config.Config{
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
	}
	deps := chatpolicy.IngressDeps{CurrentConfig: func() config.Config { return testConfig }}
	menuRenderer := testutil.NewRenderService(t, t.TempDir())
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
	deps.Bridge = bridge.New(slog.Default(), dispatcher)
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Renderer: menuTestRenderer(menuRenderer), Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	event := chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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
	ingress.HandleAdapterEvent(context.Background(), event)
	ingress.HandleAdapterEvent(context.Background(), event)

	if sender.messageCount != 1 {
		t.Fatalf("builtin menu should send once before cooldown, messages=%d", sender.messageCount)
	}
	if dispatcher.deliverCount != 0 {
		t.Fatalf("builtin menu dispatched to plugins %d times", dispatcher.deliverCount)
	}
}

func TestApplyChatPolicyLogsCooldownReplySuccess(t *testing.T) {
	t.Parallel()

	logger, stream := newIngressTestLogger(t)
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
	deps.Logger = logger
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
	deps.Bridge = bridge.New(logger, &recordingDispatcherClient{})
	deps.Menu = menuext.New(menuext.Deps{CurrentConfig: deps.CurrentConfig, Plugins: deps.Plugins, Sender: deps.OutboundSender, Logger: deps.Logger})
	ingress := chatpolicy.NewIngress(deps)

	event := chatevent.NormalizedEvent{
		Kind:             chatevent.EventKindMessage,
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

	if _, allowed := ingress.ApplyChatPolicy(context.Background(), event); !allowed {
		t.Fatal("first command should be allowed")
	}
	if _, allowed := ingress.ApplyChatPolicy(context.Background(), event); allowed {
		t.Fatal("second command should be rate limited")
	}

	summary := waitForIngressLog(t, stream, func(summary logging.Summary) bool {
		return summary.PluginID == "weather" && summary.Details["error_code"] == "platform.user_rate_limited"
	})
	if summary.Level != "warn" || summary.Source != "bridge.onebot11" {
		t.Fatalf("unexpected cooldown rejection summary: %+v", summary)
	}
	if summary.Details["policy_stage"] != "cooldown" || summary.Details["error_code"] != "platform.user_rate_limited" {
		t.Fatalf("unexpected cooldown rejection details: %#v", summary.Details)
	}

	summary = waitForIngressLog(t, stream, func(summary logging.Summary) bool {
		return summary.Details["outcome"] == "delivered" && summary.Details["target_label"] == "[测试群(20001)]" && summary.Details["plain_text"] != nil && summary.Details["plain_text"] != ""
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

func menuTestRenderer(service *render.Service) menuext.Renderer {
	return func(ctx context.Context, request menuext.RenderRequest) (string, error) {
		result, err := service.Render(ctx, render.Request{Template: "help.menu", Data: request.Data, Plugin: &render.PluginContext{Name: request.PluginName, Version: request.PluginVersion}})
		return result.ImagePath, err
	}
}
