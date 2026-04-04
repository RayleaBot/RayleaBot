package app

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"rayleabot/server/internal/adapter"
	"rayleabot/server/internal/bridge"
	"rayleabot/server/internal/config"
	"rayleabot/server/internal/dispatch"
	"rayleabot/server/internal/permission"
	"rayleabot/server/internal/plugins"
	"rayleabot/server/internal/runtime"
)

func TestCommandInfoForEventUsesDefaultLevelForOmittedPermission(t *testing.T) {
	t.Parallel()

	application := &App{
		appCore: appCore{
			Config: config.Config{
				Auth: config.AuthConfig{DefaultLevel: "group_admin"},
				Command: &config.CommandConfig{
					Prefixes: []string{"/"},
				},
			},
		},
		appPlugins: appPlugins{
			Plugins: plugins.NewCatalog([]plugins.Snapshot{{
				PluginID:          "weather",
				Valid:             true,
				RegistrationState: "installed",
				DesiredState:      "enabled",
				RuntimeState:      "running",
				Commands: []plugins.Command{{
					Name: "weather-admin",
				}},
			}}),
			commandParser: newCommandParser(config.Config{
				Command: &config.CommandConfig{Prefixes: []string{"/"}},
			}),
		},
	}

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

func TestHandleAdapterEventBlocksBlacklistedMessageBeforeBridge(t *testing.T) {
	t.Parallel()

	repo := newStubBlacklistRepo()
	repo.block("user", "bad-user")
	runtimeClient := &recordingRuntimeClient{}
	application := &App{
		appCore: appCore{Config: config.Config{}},
		appPlugins: appPlugins{
			permissionChecker: newPermissionChecker(config.Config{}, repo),
			commandParser:     newCommandParser(config.Config{}),
			Bridge:            bridge.New(slog.Default(), runtimeClient, nil, nil),
		},
	}

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

	if runtimeClient.deliverCount != 0 {
		t.Fatalf("deliverCount = %d, want 0", runtimeClient.deliverCount)
	}
}

func TestHandleAdapterEventUsesMostStrictMatchingCommandPermission(t *testing.T) {
	t.Parallel()

	runtimeClient := &recordingRuntimeClient{}
	cfg := config.Config{
		Auth: config.AuthConfig{DefaultLevel: "everyone"},
		Command: &config.CommandConfig{
			Prefixes: []string{"/"},
		},
	}
	application := &App{
		appCore: appCore{Config: cfg},
		appPlugins: appPlugins{
			permissionChecker: newPermissionChecker(cfg, nil),
			commandParser:     newCommandParser(cfg),
			Plugins: plugins.NewCatalog([]plugins.Snapshot{
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
			}),
			Bridge: bridge.New(slog.Default(), runtimeClient, nil, nil),
		},
	}

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

	if runtimeClient.deliverCount != 0 {
		t.Fatalf("deliverCount = %d, want 0", runtimeClient.deliverCount)
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
	application := &App{
		appCore: appCore{Config: cfg},
		appPlugins: appPlugins{
			permissionChecker: newPermissionChecker(cfg, nil),
			commandParser:     newCommandParser(cfg),
			outboundSender:    sender,
			Plugins: plugins.NewCatalog([]plugins.Snapshot{{
				PluginID:          "weather",
				Valid:             true,
				RegistrationState: "installed",
				DesiredState:      "enabled",
				RuntimeState:      "running",
				Commands: []plugins.Command{{
					Name:       "weather",
					Permission: "everyone",
				}},
			}}),
		},
	}
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

func TestApplyHotReloadableFieldsReloadsCommandPolicy(t *testing.T) {
	t.Parallel()

	repo := newStubBlacklistRepo()
	app := &App{
		appCore: appCore{
			Config: config.Config{
				Auth: config.AuthConfig{
					SuperAdmins:  []string{"1"},
					DefaultLevel: "everyone",
				},
				Command: &config.CommandConfig{
					Prefixes: []string{"/"},
				},
				Cooldown: &config.CooldownConfig{
					UserCommandRateLimit:  "5/1h",
					GroupCommandRateLimit: "5/1h",
					CooldownReply:         false,
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
			},
		},
		appPlugins: appPlugins{
			blacklistRepo:     repo,
			permissionChecker: newPermissionChecker(config.Config{Auth: config.AuthConfig{SuperAdmins: []string{"1"}}}, repo),
			commandParser:     newCommandParser(config.Config{Command: &config.CommandConfig{Prefixes: []string{"/"}}}),
		},
	}

	restartRequired := applyHotReloadableFields(app, config.Config{
		Auth: config.AuthConfig{
			SuperAdmins:  []string{"42"},
			DefaultLevel: "group_admin",
		},
		Command: &config.CommandConfig{
			Prefixes: []string{"!"},
		},
		Cooldown: &config.CooldownConfig{
			UserCommandRateLimit:  "1/1h",
			GroupCommandRateLimit: "2/1h",
			CooldownReply:         true,
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
	})
	if restartRequired {
		t.Fatal("restartRequired = true, want false for hot-reloadable fields")
	}
	if !app.commandParser.Parse("!ping").IsCommand {
		t.Fatal("new command prefix was not applied")
	}
	if app.commandParser.Parse("/ping").IsCommand {
		t.Fatal("old command prefix should no longer be active")
	}
	if verdict := app.permissionChecker.Check(context.Background(), "42", "member", "", &permission.CommandInfo{Permission: "super_admin"}); !verdict.Allowed {
		t.Fatalf("new super admin should bypass command checks: %#v", verdict)
	}
	if verdict := app.permissionChecker.Check(context.Background(), "1", "member", "", &permission.CommandInfo{Permission: "super_admin"}); verdict.Allowed {
		t.Fatalf("old super admin should no longer bypass command checks: %#v", verdict)
	}
	if app.Config.Storage.FileMaxBytes != 8192 || app.Config.Storage.PluginWorkDirMB != 64 {
		t.Fatalf("storage config was not hot reloaded: %+v", app.Config.Storage)
	}
	if app.Config.HTTP.TimeoutSeconds != 15 || app.Config.HTTP.MaxRetries != 2 {
		t.Fatalf("http config was not hot reloaded: %+v", app.Config.HTTP)
	}
	if len(app.Config.HTTP.AllowPrivateHosts) != 1 || app.Config.HTTP.AllowPrivateHosts[0] != "127.0.0.1" {
		t.Fatalf("http allow_private_hosts was not hot reloaded: %+v", app.Config.HTTP.AllowPrivateHosts)
	}
}

type recordingRuntimeClient struct {
	deliverCount int
}

func (r *recordingRuntimeClient) Snapshot() runtime.Snapshot {
	return runtime.Snapshot{State: runtime.StateRunning}
}

func (r *recordingRuntimeClient) DeliverEvent(_ context.Context, _ runtime.Event) (runtime.Delivery, error) {
	r.deliverCount++
	return runtime.Delivery{}, nil
}

type recordingOutboundSender struct {
	replyCount      int
	lastReplyText   string
	messageCount    int
	lastMessageText string
}

func (s *recordingOutboundSender) SendMessage(_ context.Context, action adapter.OutboundMessageSend) (adapter.SendMessageResult, error) {
	s.messageCount++
	s.lastMessageText = firstTextSegment(action.Segments)
	return adapter.SendMessageResult{MessageID: "msg-1"}, nil
}

func (s *recordingOutboundSender) SendReply(_ context.Context, action adapter.OutboundMessageReply) (adapter.SendMessageResult, error) {
	s.replyCount++
	s.lastReplyText = firstTextSegment(action.Segments)
	return adapter.SendMessageResult{MessageID: "msg-2"}, nil
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

func (s *stubBlacklistRepo) Add(context.Context, string, string, string) error {
	return nil
}

func (s *stubBlacklistRepo) Remove(context.Context, string, string) error {
	return nil
}

func (s *stubBlacklistRepo) List(context.Context, string) ([]permission.BlacklistEntry, error) {
	return nil, nil
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
	app := &App{
		appCore: appCore{
			Config: config.Config{},
			Logger: slog.Default(),
		},
		appPlugins: appPlugins{
			Plugins:    catalog,
			Dispatcher: dispatch.New(slog.Default(), nil, nil, 16),
			Runtimes:   newRuntimeRegistry(slog.Default(), runtime.Options{}),
			grantRepository: &stubLifecycleGrantRepository{
				grants: map[string][]plugins.PluginGrant{
					"weather": grants,
				},
			},
		},
	}
	controller := newPluginLifecycleController(app)
	app.pluginLifecycle = controller
	return controller, catalog
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
