package services

import (
	"context"
	adapterintake "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/intake"
	adapteroutbound "github.com/RayleaBot/RayleaBot/server/internal/bot/adapter/onebot11/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"testing"
	"time"
)

func TestApplyChatPolicyLogsCooldownReplyFailure(t *testing.T) {
	t.Parallel()

	logger, stream := newAppTestLogger()
	sender := &recordingOutboundSender{
		replyErr: &adapteroutbound.Error{Code: "adapter.send_failed", Message: "cooldown reply blocked"},
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

	event := adapterintake.NormalizedEvent{
		Kind:             adapterintake.EventKindMessage,
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
		Log: config.LogConfig{Level: "info"},
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "1/1h",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
		},
	}
	app := newTestAppState(cfg, nil)
	app.setTestEventIngress(nil, repo, nil, nil)
	app.eventStack.OutboundLimiter = outbound.NewMessageRateLimiter(cfg)
	if err := app.eventStack.OutboundLimiter.Wait(context.Background(), outbound.MessageLimitRequest{
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
		Log: config.LogConfig{Level: "info"},
		Message: config.MessageConfig{
			RateLimitPerPlugin:    "2/1h",
			RateLimitPerTarget:    "100/1s",
			CircuitBreakerSeconds: 1,
		},
	})
	if restartRequired {
		t.Fatal("restartRequired = true, want false for hot-reloadable fields")
	}
	if !app.services.EventIngress.Policy().CommandParser().Parse("!ping").IsCommand {
		t.Fatal("new command prefix was not applied")
	}
	if app.services.EventIngress.Policy().CommandParser().Parse("/ping").IsCommand {
		t.Fatal("old command prefix should no longer be active")
	}
	if verdict := app.services.EventIngress.Policy().PermissionChecker().Check(context.Background(), "42", "member", "", &permission.CommandInfo{Permission: "super_admin"}); !verdict.Allowed {
		t.Fatalf("new super admin should bypass command checks: %#v", verdict)
	}
	if verdict := app.services.EventIngress.Policy().PermissionChecker().Check(context.Background(), "1", "member", "", &permission.CommandInfo{Permission: "super_admin"}); verdict.Allowed {
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
	if err := app.eventStack.OutboundLimiter.Wait(context.Background(), outbound.MessageLimitRequest{
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

func (r *recordingDispatcherClient) Dispatch(_ context.Context, _ runtimeprotocol.Event, _ string) []dispatch.DeliveryResult {
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

func (s *recordingOutboundSender) SendMessage(_ context.Context, action adapteroutbound.OutboundMessageSend) (adapteroutbound.SendMessageResult, error) {
	s.messageCount++
	s.lastMessageText = firstTextSegment(action.Segments)
	s.lastMessageImage = firstImageSegment(action.Segments)
	return adapteroutbound.SendMessageResult{MessageID: "msg-1"}, s.messageErr
}

func (s *recordingOutboundSender) SendReply(_ context.Context, action adapteroutbound.OutboundMessageReply) (adapteroutbound.SendMessageResult, error) {
	s.replyCount++
	s.lastReplyText = firstTextSegment(action.Segments)
	s.lastReplyImage = firstImageSegment(action.Segments)
	return adapteroutbound.SendMessageResult{MessageID: "msg-2"}, s.replyErr
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
		return &adapteroutbound.Error{Code: "platform.rate_limited", Message: "outbound message rate limit exceeded"}
	}
	return nil
}

func firstTextSegment(segments []adapteroutbound.OutboundMessageSegment) string {
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

func firstImageSegment(segments []adapteroutbound.OutboundMessageSegment) string {
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
