package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/auth"
	"github.com/RayleaBot/RayleaBot/server/internal/testapp"
	"github.com/RayleaBot/RayleaBot/server/internal/testutil"
)

func TestMain(m *testing.M) {
	if err := os.Chdir(testutil.ResolveRepoPath("server")); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func newTestApp(t *testing.T, authOptions ...auth.Option) *app.App {
	return testapp.NewTestApp(t, authOptions...)
}

func newTestAppWithConfigMutation(t *testing.T, mutate func(map[string]any), authOptions ...auth.Option) (*app.App, string, string) {
	return testapp.NewTestAppWithConfigMutation(t, mutate, authOptions...)
}

func deterministicAuthOptions() []auth.Option {
	return testutil.DeterministicAuthOptions()
}

func loadConfigFixture(t *testing.T, path string) testutil.ConfigFixture {
	return testutil.LoadConfigFixture(t, path)
}

func writeYAMLConfig(t *testing.T, raw json.RawMessage) string {
	return testutil.WriteYAMLConfig(t, raw)
}

func writePersistentYAMLConfig(t *testing.T, databasePath string) string {
	return testapp.WritePersistentYAMLConfig(t, databasePath)
}

func newPersistentTestApp(t *testing.T, configPath string, now func() time.Time, sessionPrefix string) *app.App {
	return testapp.NewPersistentTestApp(t, configPath, now, sessionPrefix)
}

func closePersistentTestApp(t *testing.T, application *app.App) {
	testapp.ClosePersistentTestApp(t, application)
}

func issueExistingBootstrapLoginToken(t *testing.T, application interface{ Handler() http.Handler }) string {
	return testutil.IssueExistingBootstrapLoginToken(t, application)
}

func loadWebAPIFixtureDocument(t *testing.T, path string) testutil.WebAPIFixtureDocument {
	return testutil.LoadWebAPIFixtureDocument(t, path)
}

func performJSONRequest(t *testing.T, application interface{ Handler() http.Handler }, method, path string, body map[string]any) *httptest.ResponseRecorder {
	return testutil.PerformJSONRequest(t, application, method, path, body)
}

func decodeBody(t *testing.T, raw []byte) map[string]any {
	return testutil.DecodeBody(t, raw)
}

func readAll(t *testing.T, response *http.Response) []byte {
	return testutil.ReadAll(t, response)
}
