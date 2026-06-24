package services

import (
	"bytes"
	"context"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginconfig "github.com/RayleaBot/RayleaBot/server/internal/plugins/configstore"
	pluginkv "github.com/RayleaBot/RayleaBot/server/internal/plugins/kvstore"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteLoggerWriteAppliesRateLimit(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	application := newTestAppState(config.Config{
		Log: config.LogConfig{
			RateLimitPerPlugin: "1/1h",
		},
	}, slog.New(slog.NewJSONHandler(buffer, nil)))
	application.state.redactText = func(text string) string {
		return text
	}
	application.setTestLocalActions(
		&stubCapabilityView{capabilities: map[string][]stubCapability{
			"notice-logger": {{PluginID: "notice-logger", Capability: "logger.write"}},
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		newPluginLogLimiter(config.Config{Log: config.LogConfig{RateLimitPerPlugin: "1/1h"}}),
		nil,
	)

	if _, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_2", runtimeaction.Action{
		Kind:       "logger.write",
		LogLevel:   "info",
		LogMessage: "first log",
	}); err != nil {
		t.Fatalf("first logger.write failed: %v", err)
	}

	_, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_3", runtimeaction.Action{
		Kind:       "logger.write",
		LogLevel:   "info",
		LogMessage: "second log",
	})
	assertRuntimeErrorCode(t, err, "platform.rate_limited")
}

func TestExecuteStorageKVRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := pluginkv.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	application := newTestAppState(config.Config{
		Storage: config.StorageConfig{
			KVValueMaxBytes: 1024,
			KVTotalLimitMB:  1,
		},
	}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubCapabilityView{
			capabilities: map[string][]stubCapability{
				"notice-logger": {{
					PluginID:   "notice-logger",
					Capability: "storage.kv",
				}},
			},
		},
		nil,
		nil,
		repo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	if _, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_4", runtimeaction.Action{
		Kind:             "storage.kv",
		StorageOperation: "set",
		StorageKey:       "notice:last_join",
		StorageValue: map[string]any{
			"user_id": "3001",
			"count":   2,
		},
	}); err != nil {
		t.Fatalf("storage set failed: %v", err)
	}

	getResult, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_5", runtimeaction.Action{
		Kind:             "storage.kv",
		StorageOperation: "get",
		StorageKey:       "notice:last_join",
	})
	if err != nil {
		t.Fatalf("storage get failed: %v", err)
	}
	if exists, _ := getResult["exists"].(bool); !exists {
		t.Fatalf("expected get exists=true, got %#v", getResult)
	}

	listResult, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_6", runtimeaction.Action{
		Kind:             "storage.kv",
		StorageOperation: "list",
		StoragePrefix:    "notice:",
	})
	if err != nil {
		t.Fatalf("storage list failed: %v", err)
	}
	keys, _ := listResult["keys"].([]string)
	if len(keys) != 1 || keys[0] != "notice:last_join" {
		t.Fatalf("unexpected list keys: %#v", listResult["keys"])
	}

	deleteResult, err := application.executeLocalAction(context.Background(), "notice-logger", "req_local_7", runtimeaction.Action{
		Kind:             "storage.kv",
		StorageOperation: "delete",
		StorageKey:       "notice:last_join",
	})
	if err != nil {
		t.Fatalf("storage delete failed: %v", err)
	}
	if deleted, _ := deleteResult["deleted"].(bool); !deleted {
		t.Fatalf("expected delete deleted=true, got %#v", deleteResult)
	}
}

func TestExecuteConfigWriteDispatchesConfigChanged(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := pluginconfig.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.setTestLocalActions(
		&stubCapabilityView{capabilities: map[string][]stubCapability{
			"weather": {{PluginID: "weather", Capability: "config.write"}},
		}},
		repo,
		nil,
		nil,
		nil,
		dispatcher,
		nil,
		nil,
		nil,
		nil,
	)
	fakeRuntime := &capturingRuntime{events: make(chan runtimeprotocol.Event, 1)}
	application.eventStack.Dispatcher.Register("weather", fakeRuntime, []string{"config.changed"}, nil, 1)

	if _, err := application.executeLocalAction(context.Background(), "weather", "req_config_changed", runtimeaction.Action{
		Kind: "config.write",
		ConfigValues: map[string]any{
			"default_city": "上海",
		},
	}); err != nil {
		t.Fatalf("config.write failed: %v", err)
	}

	select {
	case event := <-fakeRuntime.events:
		if event.EventType != "config.changed" {
			t.Fatalf("unexpected config.changed event: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected config.changed event")
	}
}

func TestExecuteGovernanceActionsRejectMissingCapability(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.blacklistRepo = permission.NewSQLiteBlacklistRepository(store.Read, store.Write)
	application.whitelistRepo = permission.NewSQLiteWhitelistRepository(store.Read, store.Write)
	application.whitelistState = permission.NewSQLiteWhitelistStateRepository(store.Read, store.Write)
	application.setTestLocalActions(
		&stubCapabilityView{capabilities: map[string][]stubCapability{}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	_, err = application.executeLocalAction(context.Background(), "governance-helper", "req_governance_unauthorized", runtimeaction.Action{
		Kind: "governance.blacklist.read",
	})
	assertRuntimeErrorCode(t, err, "plugin.capability_violation")
}

func TestExecuteGovernanceActionsRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.blacklistRepo = permission.NewSQLiteBlacklistRepository(store.Read, store.Write)
	application.whitelistRepo = permission.NewSQLiteWhitelistRepository(store.Read, store.Write)
	application.whitelistState = permission.NewSQLiteWhitelistStateRepository(store.Read, store.Write)
	application.pluginStack.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		DeclaredCapabilities: []string{
			"governance.blacklist.read",
			"governance.blacklist.write",
			"governance.whitelist.read",
			"governance.whitelist.write",
			"governance.command_policy.read",
		},
		Commands: []plugins.Command{
			{Name: "forecast", Permission: "group_admin", Aliases: []string{"fc"}, CommandSource: plugins.CommandSourceManifest},
			{Name: "current", CommandSource: plugins.CommandSourceManifest},
		},
	}})
	application.setTestLocalActions(
		&stubCapabilityView{capabilities: map[string][]stubCapability{
			"governance-helper": {
				{PluginID: "governance-helper", Capability: "governance.blacklist.read"},
				{PluginID: "governance-helper", Capability: "governance.blacklist.write"},
				{PluginID: "governance-helper", Capability: "governance.whitelist.read"},
				{PluginID: "governance-helper", Capability: "governance.whitelist.write"},
				{PluginID: "governance-helper", Capability: "governance.command_policy.read"},
			},
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	blacklistWrite, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_blacklist_upsert", runtimeaction.Action{
		Kind:                "governance.blacklist.write",
		GovernanceOperation: "upsert",
		GovernanceEntryType: "user",
		GovernanceTargetID:  "1001",
		GovernanceReason:    "spam",
	})
	if err != nil {
		t.Fatalf("governance.blacklist.write upsert failed: %v", err)
	}
	if blacklistWrite["entry_type"] != "user" || blacklistWrite["target_id"] != "1001" {
		t.Fatalf("unexpected blacklist write result: %#v", blacklistWrite)
	}

	blacklistRead, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_blacklist_read", runtimeaction.Action{
		Kind: "governance.blacklist.read",
	})
	if err != nil {
		t.Fatalf("governance.blacklist.read failed: %v", err)
	}
	userEntries, _ := blacklistRead["user_entries"].([]governance.EntryResponse)
	if len(userEntries) != 1 || userEntries[0].TargetID != "1001" {
		t.Fatalf("unexpected blacklist snapshot: %#v", blacklistRead)
	}

	whitelistToggle, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_whitelist_enabled", runtimeaction.Action{
		Kind:                "governance.whitelist.write",
		GovernanceOperation: "set_enabled",
		GovernanceEnabled:   boolPointer(true),
	})
	if err != nil {
		t.Fatalf("governance.whitelist.write set_enabled failed: %v", err)
	}
	if whitelistToggle["enabled"] != true {
		t.Fatalf("unexpected whitelist toggle result: %#v", whitelistToggle)
	}

	if _, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_whitelist_upsert", runtimeaction.Action{
		Kind:                "governance.whitelist.write",
		GovernanceOperation: "upsert",
		GovernanceEntryType: "group",
		GovernanceTargetID:  "2001",
		GovernanceReason:    "approved",
	}); err != nil {
		t.Fatalf("governance.whitelist.write upsert failed: %v", err)
	}

	whitelistRead, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_whitelist_read", runtimeaction.Action{
		Kind: "governance.whitelist.read",
	})
	if err != nil {
		t.Fatalf("governance.whitelist.read failed: %v", err)
	}
	groupEntries, _ := whitelistRead["group_entries"].([]governance.EntryResponse)
	if whitelistRead["enabled"] != true || len(groupEntries) != 1 || groupEntries[0].TargetID != "2001" {
		t.Fatalf("unexpected whitelist snapshot: %#v", whitelistRead)
	}

	commandPolicy, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_command_policy", runtimeaction.Action{
		Kind: "governance.command_policy.read",
	})
	if err != nil {
		t.Fatalf("governance.command_policy.read failed: %v", err)
	}
	commands, _ := commandPolicy["commands"].([]governance.CommandPolicyEntryResponse)
	if commandPolicy["default_level"] != "everyone" || len(commands) != 2 {
		t.Fatalf("unexpected command policy: %#v", commandPolicy)
	}
	for _, command := range commands {
		if command.CommandSource != "manifest" {
			t.Fatalf("unexpected command source in policy: %#v", command)
		}
	}

	if _, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_blacklist_delete", runtimeaction.Action{
		Kind:                "governance.blacklist.write",
		GovernanceOperation: "delete",
		GovernanceEntryType: "user",
		GovernanceTargetID:  "1001",
	}); err != nil {
		t.Fatalf("governance.blacklist.write delete failed: %v", err)
	}
}

func TestExecuteGovernanceWritePublishesGovernanceChanged(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	application.blacklistRepo = permission.NewSQLiteBlacklistRepository(store.Read, store.Write)
	application.whitelistRepo = permission.NewSQLiteWhitelistRepository(store.Read, store.Write)
	application.whitelistState = permission.NewSQLiteWhitelistStateRepository(store.Read, store.Write)
	application.setTestLocalActions(
		&stubCapabilityView{capabilities: map[string][]stubCapability{
			"governance-helper": {{PluginID: "governance-helper", Capability: "governance.blacklist.write"}},
		}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	events, unsubscribe := application.services.GovernanceEvents.Subscribe(1)
	defer unsubscribe()

	if _, err := application.executeLocalAction(context.Background(), "governance-helper", "req_governance_publish", runtimeaction.Action{
		Kind:                "governance.blacklist.write",
		GovernanceOperation: "upsert",
		GovernanceEntryType: "user",
		GovernanceTargetID:  "1001",
		GovernanceReason:    "spam",
	}); err != nil {
		t.Fatalf("governance.blacklist.write upsert failed: %v", err)
	}

	select {
	case frame := <-events:
		data, ok := frame.Data.(managementevents.GenericPayload)
		if !ok || data.EventType != "governance.changed" {
			t.Fatalf("unexpected governance event: %#v", frame)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected governance.changed event")
	}
}

func TestExecuteSchedulerCreateUpsertDoesNotWriteManagementLog(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer store.Close()

	repo, err := scheduler.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}
	engine, err := scheduler.New(scheduler.Options{
		Repository: repo,
		Logger:     slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
	})
	if err != nil {
		t.Fatalf("scheduler.New: %v", err)
	}

	application := newTestAppState(config.Config{}, slog.New(slog.NewTextHandler(buffer, nil)))
	application.pluginStack.Plugins = plugincatalog.New([]plugins.Snapshot{{
		PluginID:             "weather",
		Name:                 "天气插件",
		Valid:                true,
		RegistrationState:    "installed",
		DeclaredCapabilities: []string{"scheduler.create"},
	}})
	application.setTestLocalActions(
		nil,
		nil,
		nil,
		nil,
		engine,
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	first, err := application.executeLocalAction(context.Background(), "weather", "req_sched_1", runtimeaction.Action{
		Kind:               "scheduler.create",
		SchedulerTaskID:    "daily_report",
		SchedulerLogLabel:  "每日早报",
		SchedulerCron:      "0 8 * * *",
		SchedulerEventType: "scheduler.trigger",
		SchedulerPayload: map[string]any{
			"topic": "daily_report",
		},
	})
	if err != nil {
		t.Fatalf("first scheduler.create failed: %v", err)
	}
	if first["task_id"] != "daily_report" {
		t.Fatalf("unexpected task_id: %#v", first["task_id"])
	}
	if _, ok := first["next_run"].(string); !ok {
		t.Fatalf("expected next_run string, got %#v", first["next_run"])
	}

	second, err := application.executeLocalAction(context.Background(), "weather", "req_sched_2", runtimeaction.Action{
		Kind:               "scheduler.create",
		SchedulerTaskID:    "daily_report",
		SchedulerLogLabel:  "新版早报",
		SchedulerCron:      "30 9 * * *",
		SchedulerEventType: "scheduler.trigger",
		SchedulerPayload: map[string]any{
			"topic": "daily_report_v2",
		},
	})
	if err != nil {
		t.Fatalf("second scheduler.create failed: %v", err)
	}
	if second["task_id"] != "daily_report" {
		t.Fatalf("unexpected second task_id: %#v", second["task_id"])
	}

	jobs := engine.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
	if jobs[0].JobID != "daily_report" || jobs[0].CronExpr != "30 9 * * *" {
		t.Fatalf("unexpected upserted job: %#v", jobs[0])
	}
	if jobs[0].LogLabel != "新版早报" {
		t.Fatalf("LogLabel = %q, want 新版早报", jobs[0].LogLabel)
	}
	if logs := buffer.String(); strings.Contains(logs, "定时任务已注册") {
		t.Fatalf("scheduler registration should not write management log:\n%s", logs)
	}
}
