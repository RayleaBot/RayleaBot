package testapp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/bridge"
	"github.com/RayleaBot/RayleaBot/server/internal/eventpipeline/dispatch"
	plugindiscovery "github.com/RayleaBot/RayleaBot/server/internal/plugins/discovery"
	runtimeprotocol "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/protocol"
	"github.com/RayleaBot/RayleaBot/server/internal/testutil"
)

func NewTestApp(t testing.TB, authOptions ...auth.Option) *app.App {
	t.Helper()

	application, _, _ := NewTestAppWithOptions(t, nil, nil, authOptions...)
	return application
}

func NewTestAppWithConfigMutation(t testing.TB, mutate func(map[string]any), authOptions ...auth.Option) (*app.App, string, string) {
	return NewTestAppWithOptions(t, mutate, nil, authOptions...)
}

func NewTestAppWithOptions(
	t testing.TB,
	mutate func(map[string]any),
	configureOptions func(*app.Options, string),
	authOptions ...auth.Option,
) (*app.App, string, string) {
	t.Helper()

	fixture := testutil.LoadConfigFixture(t, "../fixtures/config/ok.minimal.json")

	var input map[string]any
	if err := json.Unmarshal(fixture.Input, &input); err != nil {
		t.Fatalf("unmarshal config fixture input: %v", err)
	}
	if mutate != nil {
		mutate(input)
	}

	updated, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal config fixture input: %v", err)
	}

	configPath := testutil.WriteYAMLConfig(t, updated)
	schemaPath := testutil.RepoPath(t, "contracts", "config.user.schema.json")
	repoRoot := testutil.NewPreparedTestRuntimeRoot(t)
	builtinRoot := testutil.RepoPath(t, "plugins", "builtin")

	options := app.Options{
		ConfigPath:       configPath,
		SchemaPath:       schemaPath,
		PluginRepoRoot:   repoRoot,
		PluginSchemaPath: testutil.RepoPath(t, "contracts", "plugin-info.schema.json"),
		PluginRoots: []plugindiscovery.ScanRoot{
			{Label: "plugins/builtin", Path: builtinRoot},
			{Label: "plugins/installed", Path: filepath.Join(filepath.Dir(configPath), "..", "plugins", "installed")},
		},
		AuthOptions: authOptions,
	}
	if configureOptions != nil {
		configureOptions(&options, configPath)
	}

	application, err := app.New(options)
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}
	t.Cleanup(func() {
		if err := application.Close(); err != nil {
			t.Fatalf("close app resources: %v", err)
		}
	})

	return application, configPath, schemaPath
}

func NewPersistentTestApp(t testing.TB, configPath string, now func() time.Time, sessionPrefix string) *app.App {
	t.Helper()

	sessionCounter := 0
	repoRoot := testutil.RepoRoot(t)
	application, err := app.New(app.Options{
		ConfigPath:       configPath,
		SchemaPath:       testutil.RepoPath(t, "contracts", "config.user.schema.json"),
		PluginRepoRoot:   repoRoot,
		PluginSchemaPath: testutil.RepoPath(t, "contracts", "plugin-info.schema.json"),
		PluginRoots: []plugindiscovery.ScanRoot{
			{Label: "plugins/builtin", Path: testutil.RepoPath(t, "plugins", "builtin")},
			{Label: "plugins/installed", Path: filepath.Join(filepath.Dir(configPath), "..", "plugins", "installed")},
		},
		AuthOptions: []auth.Option{
			auth.WithClock(now),
			auth.WithSessionIDGenerator(func() (string, error) {
				sessionCounter++
				return sessionPrefix + "-" + string(rune('0'+sessionCounter)), nil
			}),
		},
	})
	if err != nil {
		t.Fatalf("app.New failed: %v", err)
	}

	return application
}

func ClosePersistentTestApp(t testing.TB, application *app.App) {
	t.Helper()

	if application != nil {
		if err := application.Close(); err != nil {
			t.Fatalf("close persistent app resources: %v", err)
		}
	}
}

func NewPersistentEventsBridge(application *app.App) *bridge.Bridge {
	return bridge.New(application.Logger(), &persistentDispatchStub{
		deliverable: true,
		results: []dispatch.DeliveryResult{{
			PluginID: "weather",
			Outcome:  dispatch.OutcomeDelivered,
		}},
	})
}

func WritePersistentYAMLConfig(t testing.TB, databasePath string) string {
	t.Helper()

	fixture := testutil.LoadConfigFixture(t, "../fixtures/config/ok.minimal.json")

	var input map[string]any
	if err := json.Unmarshal(fixture.Input, &input); err != nil {
		t.Fatalf("unmarshal config fixture input: %v", err)
	}

	database := input["database"].(map[string]any)
	database["path"] = databasePath

	return testutil.WriteYAMLConfigMap(t, input)
}

type persistentDispatchStub struct {
	deliverable bool
	results     []dispatch.DeliveryResult
}

func (s *persistentDispatchStub) HasDeliverablePlugins() bool {
	return s.deliverable
}

func (s *persistentDispatchStub) Dispatch(context.Context, runtimeprotocol.Event, string) []dispatch.DeliveryResult {
	return append([]dispatch.DeliveryResult(nil), s.results...)
}

func ResponseBodyString(t testing.TB, body map[string]any) string {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response body: %v", err)
	}
	return string(encoded)
}

func NormalizeJSONMap(t testing.TB, body map[string]any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal json map: %v", err)
	}
	var normalized map[string]any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		t.Fatalf("normalize json map: %v", err)
	}
	return normalized
}

func MustYAML(t testing.TB, value any) []byte {
	t.Helper()

	data, err := yaml.Marshal(value)
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}
	return data
}
