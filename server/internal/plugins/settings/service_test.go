package settings_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/pipeline/dispatch"
	"github.com/RayleaBot/RayleaBot/server/internal/management"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	secretssqlite "github.com/RayleaBot/RayleaBot/server/internal/platform/secrets/sqlite"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
	pluginstore "github.com/RayleaBot/RayleaBot/server/internal/plugins/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

type fixture struct {
	store      *storage.Store
	repo       *pluginstore.ConfigSQLiteRepository
	secrets    *secretssqlite.Store
	catalog    *catalog.Catalog
	dispatcher *dispatch.Dispatcher
	service    *settings.Service
	actions    *actions.Service
	router     http.Handler
	events     chan chatevent.Event
}

func newFixture(t *testing.T, configure func(*settings.Deps), controlQueueSize ...int) *fixture {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	repo, err := pluginstore.NewConfigSQLiteRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	secretStore, err := secretssqlite.NewStore(store)
	if err != nil {
		t.Fatal(err)
	}
	entry := plugins.Snapshot{
		PluginID: "weather", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running",
		DefaultConfig:    map[string]any{"count": 3, "nested": map[string]any{"old": true}, "trigger_commands": []any{"old"}},
		ManifestCommands: []plugins.Command{{ID: "query", TriggerType: "setting", SettingsKey: "trigger_commands", Permission: "everyone"}},
		Permissions:      map[string]plugins.PermissionGrant{"secret.read": {}},
	}
	cat := catalog.New([]plugins.Snapshot{entry, {PluginID: "other", Valid: true, RegistrationState: "installed"}})
	d := dispatch.New(slog.Default(), nil, nil, 128, controlQueueSize...)
	t.Cleanup(d.Close)
	events := make(chan chatevent.Event, 128)
	d.Register("weather", captureRuntime{events}, []string{"config.changed", "message.group"}, catalog.ProjectCommands(entry, entry.DefaultConfig), 1)
	deps := settings.Deps{Plugins: cat, Config: repo, Secrets: secretStore, RefreshCommands: actions.RefreshCommands(cat, d), Notify: actions.NotifyConfigChanged(d)}
	if configure != nil {
		configure(&deps)
	}
	svc, err := settings.New(deps)
	if err != nil {
		t.Fatal(err)
	}
	actionService := actions.New(actions.Deps{Settings: svc, Permissions: plugins.NewPermissionView(plugins.PermissionViewDeps{Plugins: cat})})
	handlers := management.NewPluginManagementUIHandlers(management.PluginManagementUIDeps{Plugins: cat, Settings: svc})
	router := chi.NewRouter()
	handlers.RegisterProtectedRoutes(router)
	return &fixture{store: store, repo: repo, secrets: secretStore, catalog: cat, dispatcher: d, service: svc, actions: actionService, router: router, events: events}
}

type captureRuntime struct{ events chan chatevent.Event }

func (r captureRuntime) DeliverEvent(_ context.Context, event chatevent.Event) (plugins.Delivery, error) {
	r.events <- event
	return plugins.Delivery{Result: map[string]any{"handled": true}}, nil
}
func (captureRuntime) ReadyForEvents() bool { return true }
func (captureRuntime) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}

func (f *fixture) request(method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/plugins/weather/"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	return w
}

func (f *fixture) nextEvent(t *testing.T) chatevent.Event {
	t.Helper()
	select {
	case event := <-f.events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("expected admitted config.changed event")
		return chatevent.Event{}
	}
}

func TestHTTPAndActionShareEffectiveChangesAndRealRuntimeEffects(t *testing.T) {
	t.Parallel()
	var notifications atomic.Int64
	f := newFixture(t, func(deps *settings.Deps) {
		notify := deps.Notify
		deps.Notify = func(ctx context.Context, id string, values map[string]any, keys []string) error {
			notifications.Add(1)
			return notify(ctx, id, values, keys)
		}
	})
	response := f.request(http.MethodPut, "settings", `{"values":{"count":3}}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var first management.PluginSettingsUpdateResponse
	if err := json.Unmarshal(response.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if len(first.ChangedKeys) != 0 || notifications.Load() != 0 {
		t.Fatal("same default emitted an effective change")
	}
	persisted, err := f.repo.ReadAll(context.Background(), "weather")
	if err != nil || persisted["count"] != float64(3) {
		t.Fatalf("explicit default override was lost: %#v %v", persisted, err)
	}
	entries := f.catalog.List()
	for i := range entries {
		if entries[i].PluginID == "weather" {
			entries[i].DefaultConfig["count"] = 5
		}
	}
	f.catalog.Replace(entries)
	values, err := f.service.Read(context.Background(), "weather")
	if err != nil || values["count"] != float64(3) {
		t.Fatalf("default change replaced explicit override: %#v %v", values, err)
	}

	input := map[string]any{"trigger_commands": []any{"fresh"}, "nested": map[string]any{"new": true}, "plugin_id": "other", " spaced ": "exact"}
	result, err := f.actions.Execute(context.Background(), "weather", "write", plugins.Action{Kind: "config.write", ConfigValues: input}, chatevent.Event{})
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{" spaced ", "nested", "plugin_id", "trigger_commands"}
	if !reflect.DeepEqual(result["changed_keys"], keys) {
		t.Fatalf("changed keys: %#v", result)
	}
	event := f.nextEvent(t)
	if !reflect.DeepEqual(event.PayloadFields["changed_keys"], keys) {
		t.Fatalf("notification keys: %#v", event.PayloadFields)
	}
	config := event.PayloadFields["config"].(map[string]any)
	if !reflect.DeepEqual(config["nested"], map[string]any{"new": true}) {
		t.Fatalf("top-level replacement: %#v", config)
	}
	other, err := f.repo.ReadAll(context.Background(), "other")
	if err != nil || len(other) != 0 {
		t.Fatalf("request data changed identity: %#v %v", other, err)
	}
	selected, err := f.repo.Read(context.Background(), "weather", []string{" spaced "})
	if err != nil || selected[" spaced "] != "exact" {
		t.Fatalf("key changed during persistence: %#v %v", selected, err)
	}
	projected, _ := f.catalog.Get("weather")
	if len(projected.Commands) != 1 || projected.Commands[0].Name != "fresh" {
		t.Fatalf("commands not projected: %#v", projected.Commands)
	}
	results := f.dispatcher.Dispatch(context.Background(), chatevent.Event{EventID: "query", EventType: "message.group"}, "fresh")
	if len(results) != 1 || results[0].Outcome != dispatch.OutcomeDelivered {
		t.Fatalf("fresh command not routed: %#v", results)
	}
	_ = f.nextEvent(t)
	encoded, _ := json.Marshal(map[string]any{"values": input})
	response = f.request(http.MethodPut, "settings", string(encoded))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var noop management.PluginSettingsUpdateResponse
	if err := json.Unmarshal(response.Body.Bytes(), &noop); err != nil {
		t.Fatal(err)
	}
	if len(noop.ChangedKeys) != 0 || notifications.Load() != 1 {
		t.Fatalf("cross-entry retry emitted duplicate change: %#v", noop)
	}
}

func TestCommittedFailureCanResumeAcrossHTTPAndAction(t *testing.T) {
	for _, stage := range []string{"commands", "notification"} {
		t.Run(stage, func(t *testing.T) {
			var fail atomic.Bool
			fail.Store(true)
			var refreshes atomic.Int64
			f := newFixture(t, func(deps *settings.Deps) {
				refresh, notify := deps.RefreshCommands, deps.Notify
				deps.RefreshCommands = func(ctx context.Context, id string, values map[string]any) error {
					refreshes.Add(1)
					if stage == "commands" && fail.Load() {
						return errors.New("fixture refresh failure")
					}
					return refresh(ctx, id, values)
				}
				deps.Notify = func(ctx context.Context, id string, values map[string]any, keys []string) error {
					if stage == "notification" && fail.Load() {
						return errors.New("fixture admission failure")
					}
					return notify(ctx, id, values, keys)
				}
			})
			response := f.request(http.MethodPut, "settings", `{"values":{"trigger_commands":["retry"]}}`)
			if response.Code != 409 {
				t.Fatalf("status=%d %s", response.Code, response.Body.String())
			}
			var envelope struct {
				Error struct {
					Code    string
					Details map[string]any
				}
			}
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Error.Code != errorcodes.PluginSettingsApplyFailed || envelope.Error.Details["committed"] != true || envelope.Error.Details["stage"] != stage {
				t.Fatalf("missing committed semantics: %s", response.Body.String())
			}
			stored, err := f.repo.ReadAll(context.Background(), "weather")
			if err != nil || !reflect.DeepEqual(stored["trigger_commands"], []any{"retry"}) {
				t.Fatalf("committed values missing: %#v %v", stored, err)
			}
			_, err = f.actions.Execute(context.Background(), "weather", "retry-failed", plugins.Action{Kind: "config.write", ConfigValues: map[string]any{"trigger_commands": []any{"retry"}}}, chatevent.Event{})
			var actionErr *plugins.Error
			if !errors.As(err, &actionErr) || actionErr.Code != errorcodes.PluginSettingsApplyFailed || actionErr.Details["stage"] != stage {
				t.Fatalf("action lost committed failure: %#v %v", actionErr, err)
			}
			fail.Store(false)
			result, err := f.actions.Execute(context.Background(), "weather", "retry", plugins.Action{Kind: "config.write", ConfigValues: map[string]any{"trigger_commands": []any{"retry"}}}, chatevent.Event{})
			if err != nil || !reflect.DeepEqual(result["changed_keys"], []string{"trigger_commands"}) {
				t.Fatalf("same value retry lost pending changes: %#v %v", result, err)
			}
			event := f.nextEvent(t)
			if !reflect.DeepEqual(event.PayloadFields["changed_keys"], []string{"trigger_commands"}) {
				t.Fatal(event.PayloadFields)
			}
			if stage == "notification" && refreshes.Load() != 1 {
				t.Fatal("notification retry repeated successful command refresh")
			}
			result, err = f.actions.Execute(context.Background(), "weather", "noop", plugins.Action{Kind: "config.write", ConfigValues: map[string]any{"trigger_commands": []any{"retry"}}}, chatevent.Event{})
			if err != nil || len(result["changed_keys"].([]string)) != 0 {
				t.Fatalf("pending effects not cleared: %#v %v", result, err)
			}
		})
	}
}

func TestConcurrentWritersObserveOneOrderedEffectiveSnapshot(t *testing.T) {
	t.Parallel()
	// This case verifies write ordering; reserve admission for every concurrent write.
	// Queue rejection and retry use the default capacity in their dedicated test.
	f := newFixture(t, nil, 20)
	var writes sync.WaitGroup
	for i := range 20 {
		writes.Go(func() {
			key := fmt.Sprintf("key_%02d", i)
			if i%2 == 0 {
				result := f.request(http.MethodPut, "settings", fmt.Sprintf(`{"values":{"%s":%d}}`, key, i))
				if result.Code != 200 {
					t.Errorf("HTTP write: %s", result.Body.String())
				}
			} else {
				_, err := f.actions.Execute(context.Background(), "weather", key, plugins.Action{Kind: "config.write", ConfigValues: map[string]any{key: i}}, chatevent.Event{})
				if err != nil {
					t.Error(err)
				}
			}
		})
	}
	writes.Wait()
	previous := make(map[string]any)
	for range 20 {
		event := f.nextEvent(t)
		config := event.PayloadFields["config"].(map[string]any)
		for key, value := range previous {
			if !reflect.DeepEqual(config[key], value) {
				t.Fatalf("notification lost preceding committed value %s", key)
			}
		}
		previous = config
	}
	for i := range 20 {
		if previous[fmt.Sprintf("key_%02d", i)] != float64(i) {
			t.Fatalf("missing concurrent value %d", i)
		}
	}
}

func TestCredentialsAreAtomicPrivateAndHaveNoSettingsEffects(t *testing.T) {
	t.Parallel()
	var effects atomic.Int64
	f := newFixture(t, func(deps *settings.Deps) {
		deps.Notify = func(context.Context, string, map[string]any, []string) error { effects.Add(1); return nil }
	})
	set := f.request(http.MethodPut, "secrets", `{"values":{"a":"fixture-a","b":"fixture-b"}}`)
	if set.Code != 200 || strings.Contains(set.Body.String(), "fixture-") {
		t.Fatalf("credential response: %d %s", set.Code, set.Body.String())
	}
	result, err := f.actions.Execute(context.Background(), "weather", "secret", plugins.Action{Kind: "secret.read", SecretKey: "a"}, chatevent.Event{})
	if err != nil || result["value"] != "fixture-a" {
		t.Fatalf("private read: %#v %v", result, err)
	}
	denied := actions.New(actions.Deps{Settings: f.service})
	_, err = denied.Execute(context.Background(), "weather", "denied", plugins.Action{Kind: "secret.read", SecretKey: "a"}, chatevent.Event{})
	var actionErr *plugins.Error
	if !errors.As(err, &actionErr) || actionErr.Code != errorcodes.PluginPermissionDenied {
		t.Fatalf("missing permission accepted: %v", err)
	}
	if _, exists, err := f.service.ReadSecret(context.Background(), "other", "a"); err != nil || exists {
		t.Fatalf("cross-plugin read: %v %v", exists, err)
	}
	for _, test := range []struct{ method, body string }{
		{http.MethodPut, `{"values":{"a":"changed","Bad Key":"invalid"}}`},
		{http.MethodPut, `{"values":{"a":"changed","b":""}}`},
		{http.MethodDelete, `{"keys":["a","Bad Key"]}`},
		{http.MethodDelete, `{"keys":["a","a"]}`},
	} {
		response := f.request(test.method, "secrets", test.body)
		if response.Code != 400 {
			t.Fatalf("invalid batch accepted: %d %s", response.Code, response.Body.String())
		}
		value, exists, err := f.service.ReadSecret(context.Background(), "weather", "a")
		if err != nil || !exists || value != "fixture-a" {
			t.Fatal("invalid batch partially modified credentials")
		}
	}
	noop := f.request(http.MethodPut, "secrets", `{"values":{"a":"fixture-a"}}`)
	var unchanged management.PluginSecretsUpdateResponse
	if err := json.Unmarshal(noop.Body.Bytes(), &unchanged); err != nil || noop.Code != 200 || len(unchanged.ChangedKeys) != 0 {
		t.Fatalf("same plaintext not a no-op: %s %v", noop.Body.String(), err)
	}
	deleted := f.request(http.MethodDelete, "secrets", `{"keys":["a","missing"]}`)
	var removed management.PluginSecretsUpdateResponse
	if err := json.Unmarshal(deleted.Body.Bytes(), &removed); err != nil || deleted.Code != 200 || !reflect.DeepEqual(removed.ChangedKeys, []string{"a"}) || removed.Configured["a"] || removed.Configured["missing"] {
		t.Fatalf("delete result: %s %v", deleted.Body.String(), err)
	}
	if effects.Load() != 0 {
		t.Fatal("credentials leaked into config.changed effects")
	}
}

func TestSecretBatchDatabaseFailureRollsBackAllMutations(t *testing.T) {
	t.Parallel()
	f := newFixture(t, nil)
	ctx := context.Background()
	if _, err := f.service.SetSecrets(ctx, "weather", map[string]string{"a": "original-a", "b": "original-b"}); err != nil {
		t.Fatal(err)
	}
	// Failure on the second deletion proves the first mutation is rolled back.
	if _, err := f.store.Write.ExecContext(ctx, `CREATE TRIGGER reject_secret_delete BEFORE DELETE ON secret_store WHEN OLD.key = 'plugin:weather:secret:b' BEGIN SELECT RAISE(ABORT, 'fixture failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.DeleteSecrets(ctx, "weather", []string{"a", "b"}); err == nil {
		t.Fatal("database rejection reported success")
	}
	for _, key := range []string{"a", "b"} {
		value, exists, err := f.service.ReadSecret(ctx, "weather", key)
		if err != nil || !exists || value != "original-"+key {
			t.Fatalf("partial deletion %s: %v %v", key, exists, err)
		}
	}
	// Sorted batch writes update a before the second write fails on b.
	if _, err := f.store.Write.ExecContext(ctx, `CREATE TRIGGER reject_secret_update BEFORE UPDATE ON secret_store WHEN OLD.key = 'plugin:weather:secret:b' BEGIN SELECT RAISE(ABORT, 'fixture failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.SetSecrets(ctx, "weather", map[string]string{"a": "new-a", "b": "new-b"}); err == nil {
		t.Fatal("database rejection reported success")
	}
	for _, key := range []string{"a", "b"} {
		value, exists, err := f.service.ReadSecret(ctx, "weather", key)
		if err != nil || !exists || value != "original-"+key {
			t.Fatalf("partial replacement %s: %v %v", key, exists, err)
		}
	}
}

func TestConcurrentIdenticalWritesNotifyOnce(t *testing.T) {
	t.Parallel()
	var notifications atomic.Int64
	f := newFixture(t, func(deps *settings.Deps) {
		deps.Notify = func(context.Context, string, map[string]any, []string) error { notifications.Add(1); return nil }
	})
	var writers sync.WaitGroup
	for range 20 {
		writers.Go(func() {
			if _, err := f.service.Write(t.Context(), "weather", map[string]any{"count": 7}); err != nil {
				t.Error(err)
			}
		})
	}
	writers.Wait()
	if notifications.Load() != 1 {
		t.Fatalf("identical concurrent writes notified %d times", notifications.Load())
	}
}

type queuedSettingsRuntime struct {
	started chan struct{}
	release chan struct{}
	events  chan chatevent.Event
	once    sync.Once
}

func (r *queuedSettingsRuntime) DeliverEvent(ctx context.Context, event chatevent.Event) (plugins.Delivery, error) {
	r.once.Do(func() { close(r.started) })
	select {
	case <-r.release:
	case <-ctx.Done():
		return plugins.Delivery{}, ctx.Err()
	}
	r.events <- event
	return plugins.Delivery{Result: map[string]any{"handled": true}}, nil
}
func (*queuedSettingsRuntime) ReadyForEvents() bool { return true }
func (*queuedSettingsRuntime) Snapshot() pluginruntime.Snapshot {
	return pluginruntime.Snapshot{State: pluginruntime.StateRunning}
}

func TestRealNotificationQueueFullReturnsCommittedFailureAndRetries(t *testing.T) {
	t.Parallel()
	f := newFixture(t, nil)
	runtime := &queuedSettingsRuntime{started: make(chan struct{}), release: make(chan struct{}), events: make(chan chatevent.Event, 6)}
	t.Cleanup(func() {
		select {
		case <-runtime.release:
		default:
			close(runtime.release)
		}
	})
	f.dispatcher.Register("weather", runtime, []string{"config.changed"}, nil, 1)
	event := chatevent.Event{EventID: "queued-config", EventType: "config.changed", PayloadFields: map[string]any{"config": map[string]any{"count": 0}, "changed_keys": []string{"count"}}}
	if result := f.dispatcher.DispatchToPlugin(t.Context(), "weather", event); result.Outcome != dispatch.OutcomeDelivered {
		t.Fatal(result)
	}
	<-runtime.started
	// The active delivery owns the lane; four further control events fill its queue.
	for range 4 {
		if result := f.dispatcher.DispatchToPlugin(t.Context(), "weather", event); result.Outcome != dispatch.OutcomeDelivered {
			t.Fatal(result)
		}
	}
	response := f.request(http.MethodPut, "settings", `{"values":{"count":9}}`)
	if response.Code != http.StatusConflict {
		t.Fatalf("full control queue reported success: %d %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Error struct {
			Code    string
			Details map[string]any
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Error.Code != errorcodes.PluginSettingsApplyFailed || envelope.Error.Details["committed"] != true || envelope.Error.Details["stage"] != "notification" {
		t.Fatalf("queue failure lost committed semantics: %s", response.Body.String())
	}
	stored, err := f.repo.ReadAll(t.Context(), "weather")
	if err != nil || stored["count"] != float64(9) {
		t.Fatalf("queue rejection rolled back persisted settings: %#v %v", stored, err)
	}
	close(runtime.release)
	for range 5 {
		select {
		case <-runtime.events:
		case <-time.After(time.Second):
			t.Fatal("admitted control event did not drain")
		}
	}
	result, err := f.actions.Execute(t.Context(), "weather", "retry-full-queue", plugins.Action{Kind: "config.write", ConfigValues: map[string]any{"count": 9}}, chatevent.Event{})
	if err != nil || !reflect.DeepEqual(result["changed_keys"], []string{"count"}) {
		t.Fatalf("same value retry: %#v %v", result, err)
	}
	select {
	case delivered := <-runtime.events:
		if delivered.PayloadFields["config"].(map[string]any)["count"] != float64(9) {
			t.Fatalf("retry delivered stale config: %#v", delivered.PayloadFields)
		}
	case <-time.After(time.Second):
		t.Fatal("retry failed to deliver committed settings")
	}
}

func TestActivationReplaysWritesCommittedDuringInitialization(t *testing.T) {
	t.Parallel()
	f := newFixture(t, nil)
	initial, err := f.service.Read(t.Context(), "weather")
	if err != nil {
		t.Fatal(err)
	}
	f.dispatcher.Deregister("weather")
	if _, err := f.service.Write(t.Context(), "weather", map[string]any{"count": 99}); err != nil {
		t.Fatal(err)
	}
	if err := f.service.Activate(t.Context(), "weather", initial, func() error {
		if !f.dispatcher.Register("weather", captureRuntime{f.events}, []string{"config.changed"}, nil, 1) {
			return dispatch.ErrClosed
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	event := f.nextEvent(t)
	if !reflect.DeepEqual(event.PayloadFields["changed_keys"], []string{"count"}) || event.PayloadFields["config"].(map[string]any)["count"] != float64(99) {
		t.Fatalf("initialization missed committed update: %#v", event.PayloadFields)
	}
}

func TestActivationWithLatestSnapshotCompletesPendingEffects(t *testing.T) {
	t.Parallel()
	var notifications atomic.Int64
	f := newFixture(t, func(deps *settings.Deps) {
		deps.Notify = func(context.Context, string, map[string]any, []string) error {
			notifications.Add(1)
			return errors.New("fixture admission failure")
		}
	})
	if _, err := f.service.Write(t.Context(), "weather", map[string]any{"count": 99}); err == nil {
		t.Fatal("expected committed failure")
	}
	latest, err := f.service.Read(t.Context(), "weather")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.service.Activate(t.Context(), "weather", latest, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	result, err := f.service.Write(t.Context(), "weather", map[string]any{"count": 99})
	if err != nil || len(result.ChangedKeys) != 0 || notifications.Load() != 1 {
		t.Fatalf("reload retained obsolete pending effects: %#v %v", result, err)
	}
}

func TestActivationFailureDoesNotExposeMutablePendingKeys(t *testing.T) {
	t.Parallel()
	f := newFixture(t, func(deps *settings.Deps) {
		deps.Notify = func(context.Context, string, map[string]any, []string) error {
			return errors.New("fixture notification failure")
		}
	})
	initialized, err := f.service.Read(t.Context(), "weather")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Write(t.Context(), "weather", map[string]any{"count": 9}); err != nil {
		t.Fatal(err)
	}
	err = f.service.Activate(t.Context(), "weather", initialized, func() error { return nil })
	var activationErr *settings.ApplyError
	if !errors.As(err, &activationErr) {
		t.Fatalf("activation error: %v", err)
	}
	activationErr.ChangedKeys[0] = "mutated_by_caller"
	_, err = f.service.Write(t.Context(), "weather", map[string]any{"count": 9})
	var retryErr *settings.ApplyError
	if !errors.As(err, &retryErr) || !reflect.DeepEqual(retryErr.ChangedKeys, []string{"count"}) {
		t.Fatalf("external error mutation changed pending effects: %#v %v", retryErr, err)
	}
}
