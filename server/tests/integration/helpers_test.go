package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

const (
	testManagementAuthority = testutil.TestManagementAuthority
	testManagementOrigin    = testutil.TestManagementOrigin
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
