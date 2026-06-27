package services

import (
	"context"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
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
	application.renderStack.Renderer = newRenderService(t, t.TempDir())
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
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

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
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
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
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

	_, allowed := application.applyChatPolicy(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
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

func TestHandleAdapterEventRendersBuiltinMenuPluginPrefixesAsSingleUsage(t *testing.T) {
	t.Parallel()

	sender := &recordingOutboundSender{}
	runner := &captureRenderRunner{}
	renderRoot := t.TempDir()
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	application := newTestAppState(config.Config{
		Command: &config.CommandConfig{Prefixes: []string{"/"}},
		Builtin: config.BuiltinConfig{Menu: config.BuiltinMenuConfig{
			Commands: []string{"help", "帮助"},
			Prefixes: []string{"#", "*"},
		}},
		Permission: config.PermissionConfig{DefaultLevel: "everyone"},
	}, nil)
	application.renderStack.Renderer = newRenderServiceForRepo(t, repoRoot, renderRoot, runner)
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
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
	}}), nil, sender, bridge.New(slog.Default(), &recordingDispatcherClient{}))

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
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

	if sender.replyCount != 1 || !strings.HasPrefix(sender.lastReplyImage, "file://") {
		t.Fatalf("unexpected plugin menu reply: count=%d image=%q", sender.replyCount, sender.lastReplyImage)
	}
	html := runner.lastHTML()
	for _, want := range []string{`command-usage__prefix-group`, `command-usage__prefix">#</span>`, `command-usage__prefix">*</span>`} {
		if !strings.Contains(html, want) {
			t.Fatalf("builtin plugin menu html missing %q:\n%s", want, html)
		}
	}
	if !strings.Contains(html, `command-usage__text`) {
		t.Fatalf("builtin plugin menu html missing command usage text wrapper:\n%s", html)
	}
	if got := strings.Count(html, `command-usage__name">订阅状态</span>`); got != 1 {
		t.Fatalf("command name rendered %d times, want 1:\n%s", got, html)
	}
	if strings.Contains(html, `</span><span class="command-usage__name">订阅状态</span></code><code>`) {
		t.Fatalf("command prefixes should share one usage code:\n%s", html)
	}
	if strings.Contains(html, `<span class="command-usage__args">`) {
		t.Fatalf("command without usage args should not render args span:\n%s", html)
	}
	if !strings.Contains(html, "Plugin 订阅中心 0.1.0") {
		t.Fatalf("builtin plugin menu html missing plugin footer context:\n%s", html)
	}
	if strings.Contains(html, "Plugin RayleaBot 开发版本") {
		t.Fatalf("builtin plugin menu html should not use system footer context:\n%s", html)
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
	application.renderStack.Renderer = newRenderService(t, t.TempDir())
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
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

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
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
	application.renderStack.Renderer = newRenderService(t, t.TempDir())
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
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

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
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
	application.renderStack.Renderer = newRenderService(t, t.TempDir())
	application.setTestEventIngress(plugincatalog.New([]plugins.Snapshot{{
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

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
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
	application.renderStack.Renderer = newRenderService(t, t.TempDir())
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
	}}), repo, sender, bridge.New(slog.Default(), dispatcher))

	application.handleAdapterEvent(context.Background(), adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
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
	application.renderStack.Renderer = newRenderService(t, t.TempDir())
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
	}}), nil, sender, bridge.New(slog.Default(), dispatcher))

	event := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
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

	event := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
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
		return summary.Message == "插件 weather 的命令 weather 被权限策略拒绝：用户命令触发频率限制"
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
