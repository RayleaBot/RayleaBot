package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/auth"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

const (
	testManagementAuthority = testutil.TestManagementAuthority
	testManagementOrigin    = testutil.TestManagementOrigin
)

type webAPIFixtureDocument = testutil.WebAPIFixtureDocument

func decodeBody(t *testing.T, raw []byte) map[string]any {
	return testutil.DecodeBody(t, raw)
}

func readAll(t *testing.T, response *http.Response) []byte {
	return testutil.ReadAll(t, response)
}

func loadWebAPIFixtureDocument(t *testing.T, path string) webAPIFixtureDocument {
	return testutil.LoadWebAPIFixtureDocument(t, path)
}

func performJSONRequest(t *testing.T, application interface{ Handler() http.Handler }, method, path string, body map[string]any) *httptest.ResponseRecorder {
	return testutil.PerformJSONRequest(t, application, method, path, body)
}

func performJSONRequestWithRemoteAddr(t *testing.T, application interface{ Handler() http.Handler }, method, path string, body map[string]any, remoteAddr string) *httptest.ResponseRecorder {
	return testutil.PerformJSONRequestWithRemoteAddr(t, application, method, path, body, remoteAddr)
}

func performJSONBytesRequest(t *testing.T, application interface{ Handler() http.Handler }, method, path string, payload []byte) *httptest.ResponseRecorder {
	return testutil.PerformJSONBytesRequestWithRemoteAddr(t, application, method, path, payload, "127.0.0.1:0")
}

func issueLoginToken(t *testing.T, application interface{ Handler() http.Handler }) string {
	return testutil.IssueLoginToken(t, application)
}

func issueExistingBootstrapLoginToken(t *testing.T, application interface{ Handler() http.Handler }) string {
	return testutil.IssueExistingBootstrapLoginToken(t, application)
}

// deterministicAuthOptions returns auth.Option values that produce a
// deterministic auth.Manager when passed to app.New via Options.AuthOptions.
func deterministicAuthOptions() []auth.Option {
	return testutil.DeterministicAuthOptions()
}

func newManagementTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	return testutil.NewManagementTestServer(t, handler)
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

func writePersistentYAMLConfig(t *testing.T, databasePath string) string {
	return testutil.WritePersistentYAMLConfig(t, databasePath)
}

func newPreparedTestRuntimeRoot(t *testing.T) string {
	t.Helper()

	root := testutil.NewPreparedTestRuntimeRoot(t)
	writeIntegrationInstalledPluginFixtures(t, root)
	return root
}

func writeIntegrationInstalledPluginFixtures(t *testing.T, repoRoot string) {
	t.Helper()
	testutil.WriteEchoGoPluginArtifact(t, repoRoot)
}
