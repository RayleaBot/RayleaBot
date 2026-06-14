package integration

import (
	"encoding/json"
	"net/http"
	"os"
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

func repoRootPath(t *testing.T) string {
	t.Helper()
	return testutil.RepoRoot(t)
}

func loadConfigFixture(t *testing.T, path string) testutil.ConfigFixture {
	return testutil.LoadConfigFixture(t, path)
}

func writeYAMLConfig(t *testing.T, raw json.RawMessage) string {
	return testutil.WriteYAMLConfig(t, raw)
}

func newPreparedTestRuntimeRoot(t *testing.T) string {
	return testutil.NewPreparedTestRuntimeRoot(t)
}

func newDeterministicAuthManagerWithRepository(t *testing.T, repo auth.Repository) *auth.Manager {
	return testutil.NewDeterministicAuthManagerWithRepository(t, repo)
}

type stubAuthRepository = testutil.StubAuthRepository
