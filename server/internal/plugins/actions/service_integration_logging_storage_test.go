package actions_test

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	permissionsqlite "github.com/RayleaBot/RayleaBot/server/internal/bot/permission/sqlite"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	managementevents "github.com/RayleaBot/RayleaBot/server/internal/management/events"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	localaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
	pluginstore "github.com/RayleaBot/RayleaBot/server/internal/plugins/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/scheduler"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestExecuteLoggerWriteAppliesRateLimit(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	testConfig := config.Config{
		Log: config.LogConfig{
			RateLimitPerPlugin: "1/1h",
		},
	}
	deps := localaction.Deps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = slog.New(slog.NewJSONHandler(buffer, nil))
	deps.RedactText = func(text string) string {
		return text
	}
	deps.Permissions = &scopedPermissionView{permissions: map[string][]stubPermission{
		"notice-logger": {{PluginID: "notice-logger", Permission: "logger.write"}},
	}}
	deps.PluginLogLimiter = localaction.NewPluginLogLimiter(config.Config{Log: config.LogConfig{RateLimitPerPlugin: "1/1h"}})
	application := localaction.New(deps)

	if _, err := application.Execute(context.Background(), "notice-logger", "req_local_2", plugins.Action{
		Kind:       "logger.write",
		LogLevel:   "info",
		LogMessage: "first log",
	}, chatevent.Event{}); err != nil {
		t.Fatalf("first logger.write failed: %v", err)
	}

	_, err := application.Execute(context.Background(), "notice-logger", "req_local_3", plugins.Action{
		Kind:       "logger.write",
		LogLevel:   "info",
		LogMessage: "second log",
	}, chatevent.Event{})
	assertRuntimeErrorCode(t, err, "platform.rate_limited")
}

func TestExecuteStorageKVRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer func(release func() error) { _ = release() }(store.Close)

	repo, err := pluginstore.NewKVSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	testConfig := config.Config{
		Storage: config.StorageConfig{
			KVValueMaxBytes: 1024,
			KVTotalLimitMB:  1,
		},
	}
	deps := localaction.Deps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	deps.Permissions = &scopedPermissionView{
		permissions: map[string][]stubPermission{
			"notice-logger": {{
				PluginID:   "notice-logger",
				Permission: "storage.kv",
			}},
		},
	}
	deps.PluginKV = repo
	application := localaction.New(deps)

	if _, err := application.Execute(context.Background(), "notice-logger", "req_local_4", plugins.Action{
		Kind:             "storage.kv",
		StorageOperation: "set",
		StorageKey:       "notice:last_join",
		StorageValue: map[string]any{
			"user_id": "3001",
			"count":   2,
		},
	}, chatevent.Event{}); err != nil {
		t.Fatalf("storage set failed: %v", err)
	}

	getResult, err := application.Execute(context.Background(), "notice-logger", "req_local_5", plugins.Action{
		Kind:             "storage.kv",
		StorageOperation: "get",
		StorageKey:       "notice:last_join",
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("storage get failed: %v", err)
	}
	if exists, _ := getResult["exists"].(bool); !exists {
		t.Fatalf("expected get exists=true, got %#v", getResult)
	}

	listResult, err := application.Execute(context.Background(), "notice-logger", "req_local_6", plugins.Action{
		Kind:             "storage.kv",
		StorageOperation: "list",
		StoragePrefix:    "notice:",
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("storage list failed: %v", err)
	}
	keys, _ := listResult["keys"].([]string)
	if len(keys) != 1 || keys[0] != "notice:last_join" {
		t.Fatalf("unexpected list keys: %#v", listResult["keys"])
	}

	deleteResult, err := application.Execute(context.Background(), "notice-logger", "req_local_7", plugins.Action{
		Kind:             "storage.kv",
		StorageOperation: "delete",
		StorageKey:       "notice:last_join",
	}, chatevent.Event{})
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
	defer func(release func() error) { _ = release() }(store.Close)

	repo, err := pluginstore.NewConfigSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}

	dispatcher := dispatch.New(slog.Default(), nil, nil, 16)
	t.Cleanup(dispatcher.Close)
	testConfig := config.Config{}
	deps := localaction.Deps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	catalogForActions := plugincatalog.New([]plugins.Snapshot{{PluginID: "weather", Valid: true, RegistrationState: "installed"}})
	deps.Permissions = &scopedPermissionView{permissions: map[string][]stubPermission{
		"weather": {{PluginID: "weather", Permission: "config.write"}},
	}}
	settingsService, settingsErr := settings.New(settings.Deps{Plugins: catalogForActions, Config: repo, RefreshCommands: localaction.RefreshCommands(catalogForActions, dispatcher), Notify: localaction.NotifyConfigChanged(dispatcher)})
	if settingsErr != nil {
		t.Fatal(settingsErr)
	}
	deps.Settings = settingsService
	application := localaction.New(deps)
	fakeRuntime := &testutil.EventRuntime{Events: make(chan chatevent.Event, 1)}
	dispatcher.Register("weather", fakeRuntime, []string{"config.changed"}, nil, 1)

	if _, err := application.Execute(context.Background(), "weather", "req_config_changed", plugins.Action{
		Kind: "config.write",
		ConfigValues: map[string]any{
			"default_city": "上海",
		},
	}, chatevent.Event{}); err != nil {
		t.Fatalf("config.write failed: %v", err)
	}

	select {
	case event := <-fakeRuntime.Events:
		if event.EventType != "config.changed" {
			t.Fatalf("unexpected config.changed event: %#v", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected config.changed event")
	}
}

func TestExecuteGovernanceActionsRejectMissingPermission(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer func(release func() error) { _ = release() }(store.Close)

	testConfig := config.Config{}
	deps := localaction.Deps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	blacklistRepo := permissionsqlite.NewAccessListRepository(store.Read, store.Write, permission.ListBlacklist)
	whitelistRepo := permissionsqlite.NewAccessListRepository(store.Read, store.Write, permission.ListWhitelist)
	whitelistState := permissionsqlite.NewWhitelistStateRepository(store.Read, store.Write)
	deps.Permissions = &scopedPermissionView{permissions: map[string][]stubPermission{}}
	governanceEvents := managementevents.NewGovernanceService()
	deps.Governance = governance.NewService(governance.Deps{CurrentConfig: deps.CurrentConfig, BlacklistRepo: blacklistRepo, WhitelistRepo: whitelistRepo, WhitelistState: whitelistState, NotifyChanged: governanceEvents.PublishChanged})
	application := localaction.New(deps)

	_, err = application.Execute(context.Background(), "governance-helper", "req_governance_unauthorized", plugins.Action{
		Kind: "governance.blacklist.read",
	}, chatevent.Event{})
	assertRuntimeErrorCode(t, err, "plugin.permission_denied")
}

func TestExecuteGovernanceActionsRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer func(release func() error) { _ = release() }(store.Close)

	testConfig := config.Config{}
	deps := localaction.Deps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	blacklistRepo := permissionsqlite.NewAccessListRepository(store.Read, store.Write, permission.ListBlacklist)
	whitelistRepo := permissionsqlite.NewAccessListRepository(store.Read, store.Write, permission.ListWhitelist)
	whitelistState := permissionsqlite.NewWhitelistStateRepository(store.Read, store.Write)
	catalogForActions := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "Weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "enabled",
		Permissions: map[string]plugins.PermissionGrant{
			"governance.blacklist.read": {}, "governance.blacklist.write": {},
			"governance.whitelist.read": {}, "governance.whitelist.write": {},
			"governance.command_policy.read": {},
		},
		Commands: []plugins.Command{
			{ID: "forecast", Name: "forecast", DisplayName: "forecast", TriggerType: "exact", TriggerNames: []string{"forecast", "fc"}, Permission: "group_admin", Aliases: []string{"fc"}},
			{ID: "current", Name: "current", DisplayName: "current", TriggerType: "exact", TriggerNames: []string{"current"}},
		},
	}})
	deps.Permissions = &scopedPermissionView{permissions: map[string][]stubPermission{
		"governance-helper": {
			{PluginID: "governance-helper", Permission: "governance.blacklist.read"},
			{PluginID: "governance-helper", Permission: "governance.blacklist.write"},
			{PluginID: "governance-helper", Permission: "governance.whitelist.read"},
			{PluginID: "governance-helper", Permission: "governance.whitelist.write"},
			{PluginID: "governance-helper", Permission: "governance.command_policy.read"},
		},
	}}
	governanceEvents := managementevents.NewGovernanceService()
	deps.Governance = governance.NewService(governance.Deps{CurrentConfig: deps.CurrentConfig, BlacklistRepo: blacklistRepo, WhitelistRepo: whitelistRepo, WhitelistState: whitelistState, NotifyChanged: governanceEvents.PublishChanged, Plugins: catalogForActions})
	application := localaction.New(deps)

	blacklistWrite, err := application.Execute(context.Background(), "governance-helper", "req_governance_blacklist_upsert", plugins.Action{
		Kind:                "governance.blacklist.write",
		GovernanceOperation: "upsert",
		GovernanceScope:     chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"},
		GovernanceEntryType: "user",
		GovernanceTargetID:  "1001",
		GovernanceReason:    "spam",
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("governance.blacklist.write upsert failed: %v", err)
	}
	if blacklistWrite["entry_type"] != "user" || blacklistWrite["target_id"] != "1001" {
		t.Fatalf("unexpected blacklist write result: %#v", blacklistWrite)
	}

	blacklistRead, err := application.Execute(context.Background(), "governance-helper", "req_governance_blacklist_read", plugins.Action{
		Kind: "governance.blacklist.read",
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("governance.blacklist.read failed: %v", err)
	}
	userEntries, _ := blacklistRead["user_entries"].([]governance.EntryResponse)
	if len(userEntries) != 1 || userEntries[0].TargetID != "1001" {
		t.Fatalf("unexpected blacklist snapshot: %#v", blacklistRead)
	}

	whitelistToggle, err := application.Execute(context.Background(), "governance-helper", "req_governance_whitelist_enabled", plugins.Action{
		Kind:                "governance.whitelist.write",
		GovernanceOperation: "set_enabled",
		GovernanceEnabled:   boolPointer(true),
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("governance.whitelist.write set_enabled failed: %v", err)
	}
	if whitelistToggle["enabled"] != true {
		t.Fatalf("unexpected whitelist toggle result: %#v", whitelistToggle)
	}

	if _, err := application.Execute(context.Background(), "governance-helper", "req_governance_whitelist_upsert", plugins.Action{
		Kind:                "governance.whitelist.write",
		GovernanceOperation: "upsert",
		GovernanceScope:     chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"},
		GovernanceEntryType: "group",
		GovernanceTargetID:  "2001",
		GovernanceReason:    "approved",
	}, chatevent.Event{}); err != nil {
		t.Fatalf("governance.whitelist.write upsert failed: %v", err)
	}

	whitelistRead, err := application.Execute(context.Background(), "governance-helper", "req_governance_whitelist_read", plugins.Action{
		Kind: "governance.whitelist.read",
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("governance.whitelist.read failed: %v", err)
	}
	groupEntries, _ := whitelistRead["group_entries"].([]governance.EntryResponse)
	if whitelistRead["enabled"] != true || len(groupEntries) != 1 || groupEntries[0].TargetID != "2001" {
		t.Fatalf("unexpected whitelist snapshot: %#v", whitelistRead)
	}

	commandPolicy, err := application.Execute(context.Background(), "governance-helper", "req_governance_command_policy", plugins.Action{
		Kind: "governance.command_policy.read",
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("governance.command_policy.read failed: %v", err)
	}
	commands, _ := commandPolicy["commands"].([]governance.CommandPolicyEntryResponse)
	if commandPolicy["default_level"] != "everyone" || len(commands) != 2 {
		t.Fatalf("unexpected command policy: %#v", commandPolicy)
	}
	for _, command := range commands {
		if command.CommandID == "" || command.Trigger.Type != "exact" {
			t.Fatalf("unexpected command identity in policy: %#v", command)
		}
	}

	if _, err := application.Execute(context.Background(), "governance-helper", "req_governance_blacklist_delete", plugins.Action{
		Kind:                "governance.blacklist.write",
		GovernanceOperation: "delete",
		GovernanceScope:     chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"},
		GovernanceEntryType: "user",
		GovernanceTargetID:  "1001",
	}, chatevent.Event{}); err != nil {
		t.Fatalf("governance.blacklist.write delete failed: %v", err)
	}
}

func TestExecuteGovernanceWritePublishesGovernanceChanged(t *testing.T) {
	t.Parallel()

	store, err := storage.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("storage.Open: %v", err)
	}
	defer func(release func() error) { _ = release() }(store.Close)

	testConfig := config.Config{}
	deps := localaction.Deps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	blacklistRepo := permissionsqlite.NewAccessListRepository(store.Read, store.Write, permission.ListBlacklist)
	whitelistRepo := permissionsqlite.NewAccessListRepository(store.Read, store.Write, permission.ListWhitelist)
	whitelistState := permissionsqlite.NewWhitelistStateRepository(store.Read, store.Write)
	deps.Permissions = &scopedPermissionView{permissions: map[string][]stubPermission{
		"governance-helper": {{PluginID: "governance-helper", Permission: "governance.blacklist.write"}},
	}}
	governanceEvents := managementevents.NewGovernanceService()
	deps.Governance = governance.NewService(governance.Deps{CurrentConfig: deps.CurrentConfig, BlacklistRepo: blacklistRepo, WhitelistRepo: whitelistRepo, WhitelistState: whitelistState, NotifyChanged: governanceEvents.PublishChanged})
	application := localaction.New(deps)

	events, unsubscribe := governanceEvents.Subscribe(1)
	defer unsubscribe()

	if _, err := application.Execute(context.Background(), "governance-helper", "req_governance_publish", plugins.Action{
		Kind:                "governance.blacklist.write",
		GovernanceOperation: "upsert",
		GovernanceScope:     chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"},
		GovernanceEntryType: "user",
		GovernanceTargetID:  "1001",
		GovernanceReason:    "spam",
	}, chatevent.Event{}); err != nil {
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
	defer func(release func() error) { _ = release() }(store.Close)

	repo, err := scheduler.NewSQLiteRepository(store)
	if err != nil {
		t.Fatalf("NewSQLiteRepository: %v", err)
	}
	engine, err := scheduler.New(scheduler.Options{
		Repository: repo,
		Logger:     slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		Timezone:   "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("scheduler.New: %v", err)
	}

	testConfig := config.Config{}
	deps := localaction.Deps{CurrentConfig: func() config.Config { return testConfig }}
	deps.Logger = slog.New(slog.NewTextHandler(buffer, nil))
	catalogForActions := plugincatalog.New([]plugins.Snapshot{{
		PluginID:          "weather",
		Name:              "天气插件",
		Valid:             true,
		RegistrationState: "installed",
		Permissions:       map[string]plugins.PermissionGrant{"scheduler.create": {}},
	}})
	deps.Permissions = plugins.NewPermissionView(plugins.PermissionViewDeps{Plugins: catalogForActions})
	deps.Scheduler = localaction.Scheduler(engine)
	application := localaction.New(deps)

	first, err := application.Execute(context.Background(), "weather", "req_sched_1", plugins.Action{
		Kind:               "scheduler.create",
		SchedulerTaskID:    "daily_report",
		SchedulerLogLabel:  "每日早报",
		SchedulerCron:      "0 8 * * *",
		SchedulerEventType: "scheduler.trigger",
		SchedulerPayload: map[string]any{
			"topic": "daily_report",
		},
	}, chatevent.Event{})
	if err != nil {
		t.Fatalf("first scheduler.create failed: %v", err)
	}
	if first["task_id"] != "daily_report" {
		t.Fatalf("unexpected task_id: %#v", first["task_id"])
	}
	if _, ok := first["next_run"].(string); !ok {
		t.Fatalf("expected next_run string, got %#v", first["next_run"])
	}

	second, err := application.Execute(context.Background(), "weather", "req_sched_2", plugins.Action{
		Kind:               "scheduler.create",
		SchedulerTaskID:    "daily_report",
		SchedulerLogLabel:  "新版早报",
		SchedulerCron:      "30 9 * * *",
		SchedulerEventType: "scheduler.trigger",
		SchedulerPayload: map[string]any{
			"topic": "daily_report_v2",
		},
	}, chatevent.Event{})
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
	if logs := buffer.String(); logs != "" {
		t.Fatalf("scheduler registration should not write management log:\n%s", logs)
	}
}
