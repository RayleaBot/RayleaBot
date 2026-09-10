package chatpolicy_test

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	menuext "github.com/RayleaBot/RayleaBot/server/internal/bot/menu"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/chatpolicy"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/outbound"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/pagination"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestApplyChatPolicyLogsCooldownReplyFailure(t *testing.T) {
	t.Parallel()

	logger, stream := newIngressTestLogger(t)
	sender := &recordingOutboundSender{
		replyErr: &chatevent.SendError{Code: "adapter.send_failed", Message: "cooldown reply blocked"},
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

	if _, allowed := ingress.ApplyChatPolicy(context.Background(), event); !allowed {
		t.Fatal("first command should be allowed")
	}
	if _, allowed := ingress.ApplyChatPolicy(context.Background(), event); allowed {
		t.Fatal("second command should be rate limited")
	}

	summary := waitForIngressLog(t, stream, func(summary logging.Summary) bool {
		return summary.PluginID == "weather" && summary.Details["error_code"] == "platform.user_rate_limited"
	})
	if summary.Level != "warn" {
		t.Fatalf("unexpected cooldown rejection level: got %q want warn", summary.Level)
	}
	if summary.Details["policy_stage"] != "cooldown" || summary.Details["error_code"] != "platform.user_rate_limited" {
		t.Fatalf("unexpected cooldown rejection details: %#v", summary.Details)
	}

	summary = waitForIngressLog(t, stream, func(summary logging.Summary) bool {
		return summary.Message == "消息发送失败" && summary.Details["target_label"] == "[测试群(20001)]" && summary.Details["reason"] == "cooldown reply blocked"
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

type recordingDispatcherClient struct {
	deliverCount int
}

func (r *recordingDispatcherClient) HasDeliverablePlugins() bool {
	return true
}

func (r *recordingDispatcherClient) Dispatch(_ context.Context, _ chatevent.Event, _ string) []dispatch.DeliveryResult {
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

func (s *recordingOutboundSender) SendMessage(_ context.Context, action chatevent.OutboundMessageSend) (chatevent.SendMessageResult, error) {
	s.messageCount++
	s.lastMessageText = firstTextSegment(action.Segments)
	s.lastMessageImage = firstImageSegment(action.Segments)
	return chatevent.SendMessageResult{MessageID: "msg-1"}, s.messageErr
}

func (s *recordingOutboundSender) SendReply(_ context.Context, action chatevent.OutboundMessageReply) (chatevent.SendMessageResult, error) {
	s.replyCount++
	s.lastReplyText = firstTextSegment(action.Segments)
	s.lastReplyImage = firstImageSegment(action.Segments)
	return chatevent.SendMessageResult{MessageID: "msg-2"}, s.replyErr
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
		return &chatevent.SendError{Code: "platform.rate_limited", Message: "outbound message rate limit exceeded"}
	}
	return nil
}

func firstTextSegment(segments []chatevent.MessageSegment) string {
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

func firstImageSegment(segments []chatevent.MessageSegment) string {
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

func newIngressTestLogger(t *testing.T) (*slog.Logger, *logging.Stream) {
	t.Helper()
	stream := logging.NewStream(16)
	t.Cleanup(stream.Close)
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

func waitForIngressLog(t *testing.T, stream *logging.Stream, match func(logging.Summary) bool) logging.Summary {
	t.Helper()
	events, cancel := stream.Subscribe(16)
	defer cancel()
	for _, entry := range stream.Snapshot() {
		if match(entry) {
			return entry
		}
	}
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for {
		select {
		case entry := <-events:
			if match(entry) {
				return entry
			}
		case <-timeout.C:
			t.Fatal("timed out waiting for ingress log")
			return logging.Summary{}
		}
	}
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

func (s *stubBlacklistRepo) Contains(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (bool, error) {
	if entries, ok := s.blocked[entryType]; ok {
		return entries[targetID], nil
	}
	return false, nil
}

func (s *stubBlacklistRepo) Get(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (permission.Entry, error) {
	if blocked, _ := s.Contains(context.Background(), scope, entryType, targetID); blocked {
		return permission.Entry{
			EntryType: entryType,
			TargetID:  targetID,
			Reason:    "blocked",
			CreatedAt: "2026-04-19T00:00:00Z",
		}, nil
	}
	return permission.Entry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubBlacklistRepo) Add(context.Context, chatevent.IdentityScope, string, string, string) error {
	return nil
}

func (s *stubBlacklistRepo) Remove(context.Context, chatevent.IdentityScope, string, string) error {
	return nil
}

func (s *stubBlacklistRepo) List(context.Context, string) ([]permission.Entry, error) {
	return nil, nil
}

type stubWhitelistRepo struct {
	allowed map[string]map[string]bool
}

func newStubWhitelistRepo() *stubWhitelistRepo {
	return &stubWhitelistRepo{allowed: make(map[string]map[string]bool)}
}

func (s *stubWhitelistRepo) Contains(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (bool, error) {
	if entries, ok := s.allowed[entryType]; ok {
		return entries[targetID], nil
	}
	return false, nil
}

func (s *stubWhitelistRepo) Get(_ context.Context, scope chatevent.IdentityScope, entryType, targetID string) (permission.Entry, error) {
	if allowed, _ := s.Contains(context.Background(), scope, entryType, targetID); allowed {
		return permission.Entry{
			EntryType: entryType,
			TargetID:  targetID,
			Reason:    "allowed",
			CreatedAt: "2026-04-19T00:00:00Z",
		}, nil
	}
	return permission.Entry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubWhitelistRepo) Add(context.Context, chatevent.IdentityScope, string, string, string) error {
	return nil
}

func (s *stubWhitelistRepo) Remove(context.Context, chatevent.IdentityScope, string, string) error {
	return nil
}

func (s *stubWhitelistRepo) List(context.Context, string) ([]permission.Entry, error) {
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

func (s *stubBlacklistRepo) Page(ctx context.Context, query pagination.Query, entryType string) (permission.EntryPage, error) {
	users, err := s.List(ctx, "user")
	if err != nil {
		return permission.EntryPage{}, err
	}
	groups, err := s.List(ctx, "group")
	if err != nil {
		return permission.EntryPage{}, err
	}
	all := append(users, groups...)
	filtered := make([]permission.Entry, 0, len(all))
	for _, item := range all {
		if (entryType == "" || item.EntryType == entryType) && pagination.Matches(query.Text, item.TargetID, item.Reason) {
			filtered = append(filtered, item)
		}
	}
	items, meta := pagination.Slice(filtered, query)
	return permission.EntryPage{Items: items, Total: meta.Total, EntryCount: len(all)}, nil
}

func (s *stubWhitelistRepo) Page(ctx context.Context, query pagination.Query, entryType string) (permission.EntryPage, error) {
	users, err := s.List(ctx, "user")
	if err != nil {
		return permission.EntryPage{}, err
	}
	groups, err := s.List(ctx, "group")
	if err != nil {
		return permission.EntryPage{}, err
	}
	all := append(users, groups...)
	filtered := make([]permission.Entry, 0, len(all))
	for _, item := range all {
		if (entryType == "" || item.EntryType == entryType) && pagination.Matches(query.Text, item.TargetID, item.Reason) {
			filtered = append(filtered, item)
		}
	}
	items, meta := pagination.Slice(filtered, query)
	return permission.EntryPage{Items: items, Total: meta.Total, EntryCount: len(all)}, nil
}
