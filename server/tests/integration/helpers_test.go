package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/testutil"
)

func TestMain(m *testing.M) {
	if err := os.Chdir(testutil.ResolveRepoPath("server")); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func decodeBody(t *testing.T, raw []byte) map[string]any {
	return testutil.DecodeBody(t, raw)
}

func issueLoginToken(t *testing.T, application interface{ Handler() http.Handler }) string {
	return testutil.IssueLoginToken(t, application)
}

func websocketURL(httpURL string) string {
	return testutil.WebSocketURL(httpURL)
}

func loadConfigFixture(t *testing.T, path string) testutil.ConfigFixture {
	return testutil.LoadConfigFixture(t, path)
}

func writeYAMLConfig(t *testing.T, raw json.RawMessage) string {
	return testutil.WriteYAMLConfig(t, raw)
}

func newPreparedTestRuntimeRoot(t *testing.T) string {
	t.Helper()

	root := testutil.NewPreparedTestRuntimeRoot(t)
	writeIntegrationBuiltinPluginFixtures(t, root)
	return root
}

func writeIntegrationBuiltinPluginFixtures(t *testing.T, repoRoot string) {
	t.Helper()

	pluginDir := filepath.Join(repoRoot, "plugins", "builtin", "fixture-echo")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("mkdir builtin plugin fixture: %v", err)
	}

	manifest := map[string]any{
		"id":                      "raylea.echo",
		"name":                    "Fixture Echo",
		"version":                 "0.1.0",
		"manifest_version":        "1",
		"plugin_protocol_version": "1",
		"type":                    "managed_runtime",
		"runtime":                 "python",
		"entry":                   "main.py",
		"license":                 "MIT",
		"description":             "Fixture plugin used by server integration tests.",
		"author":                  "raylea",
		"capabilities":            []any{"event.subscribe", "message.send"},
		"commands": []any{
			map[string]any{
				"name":        "echo",
				"description": "Echo fixture command.",
				"usage":       "/echo <text>",
				"permission":  "everyone",
			},
		},
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal builtin plugin fixture manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "info.json"), append(manifestBytes, '\n'), 0o644); err != nil {
		t.Fatalf("write builtin plugin fixture manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "main.py"), []byte("print('fixture echo')\n"), 0o644); err != nil {
		t.Fatalf("write builtin plugin fixture entry: %v", err)
	}
}

func newDeterministicAuthManagerWithRepository(t *testing.T, repo auth.Repository) *auth.Manager {
	return testutil.NewDeterministicAuthManagerWithRepository(t, repo)
}

type stubAuthRepository = testutil.StubAuthRepository
