package integration

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	permissionsqlite "github.com/RayleaBot/RayleaBot/server/internal/bot/permission/sqlite"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestSetupStatusReportsBootstrapState(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)

	before := performJSONRequest(t, application, http.MethodGet, "/api/setup/status", nil)
	if before.Code != http.StatusOK {
		t.Fatalf("unexpected pre-bootstrap status: got %d want 200", before.Code)
	}
	beforeBody := decodeBody(t, before.Body.Bytes())
	if beforeBody["initialized"] != false {
		t.Fatalf("expected initialized=false before bootstrap, got %#v", beforeBody["initialized"])
	}

	setupFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.setup-admin.yaml"))
	afterFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.setup-status.yaml"))
	setup := performJSONRequest(t, application, setupFixture.Request.Method, setupFixture.Request.Path, setupFixture.Request.Body)
	if setup.Code != setupFixture.Response.Status {
		t.Fatalf("unexpected bootstrap status: got %d want %d", setup.Code, setupFixture.Response.Status)
	}

	after := performJSONRequest(t, application, http.MethodGet, "/api/setup/status", nil)
	if after.Code != afterFixture.Response.Status {
		t.Fatalf("unexpected post-bootstrap status: got %d want %d", after.Code, afterFixture.Response.Status)
	}
	if got := decodeBody(t, after.Body.Bytes()); got["initialized"] != afterFixture.Response.Body["initialized"] {
		t.Fatalf("unexpected setup status body: got %#v want %#v", got, afterFixture.Response.Body)
	}
}

func TestSessionLogoutRevokesCurrentToken(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodDelete, server.URL+"/api/session", nil)
	if err != nil {
		t.Fatalf("create logout request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform logout request: %v", err)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected logout status: got %d want 204", response.StatusCode)
	}

	protected, err := http.NewRequest(http.MethodGet, server.URL+"/api/plugins", nil)
	if err != nil {
		t.Fatalf("create protected request: %v", err)
	}
	protected.Header.Set("Authorization", "Bearer "+token)
	protectedResp, err := server.Client().Do(protected)
	if err != nil {
		t.Fatalf("perform protected request: %v", err)
	}
	defer func(release func() error) { _ = release() }(protectedResp.Body.Close)
	if protectedResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected protected status after logout: got %d want 401", protectedResp.StatusCode)
	}
}

func TestSystemStatusAndShutdownHandlers(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	statusReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/system/status", nil)
	if err != nil {
		t.Fatalf("create system status request: %v", err)
	}
	statusReq.Header.Set("Authorization", "Bearer "+token)
	statusResp, err := server.Client().Do(statusReq)
	if err != nil {
		t.Fatalf("perform system status request: %v", err)
	}
	defer func(release func() error) { _ = release() }(statusResp.Body.Close)
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected system status code: got %d want 200", statusResp.StatusCode)
	}
	statusBody := decodeBody(t, readAll(t, statusResp))
	if statusBody["status"] != "running" {
		t.Fatalf("unexpected system status: %#v", statusBody["status"])
	}
	if _, ok := statusBody["adapters"].([]any); !ok {
		t.Fatalf("expected adapters array, got %#v", statusBody["adapters"])
	}
	if _, ok := statusBody["active_plugins"].(float64); !ok {
		t.Fatalf("expected active_plugins number, got %#v", statusBody["active_plugins"])
	}
	if _, ok := statusBody["running_plugins"].(float64); !ok {
		t.Fatalf("expected running_plugins number, got %#v", statusBody["running_plugins"])
	}
	if _, ok := statusBody["failed_plugins"].(float64); !ok {
		t.Fatalf("expected failed_plugins number, got %#v", statusBody["failed_plugins"])
	}
	if _, ok := statusBody["db_schema_version"].(string); !ok {
		t.Fatalf("expected db_schema_version string, got %#v", statusBody["db_schema_version"])
	}
	if _, ok := statusBody["uptime_seconds"].(float64); !ok {
		t.Fatalf("expected uptime_seconds number, got %#v", statusBody["uptime_seconds"])
	}

	shutdownFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.system-shutdown.yaml"))
	shutdownReq, err := http.NewRequest(http.MethodPost, server.URL+shutdownFixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create system shutdown request: %v", err)
	}
	shutdownReq.Header.Set("Authorization", "Bearer "+token)
	shutdownResp, err := server.Client().Do(shutdownReq)
	if err != nil {
		t.Fatalf("perform system shutdown request: %v", err)
	}
	defer func(release func() error) { _ = release() }(shutdownResp.Body.Close)
	if shutdownResp.StatusCode != shutdownFixture.Response.Status {
		t.Fatalf("unexpected system shutdown status: got %d want %d", shutdownResp.StatusCode, shutdownFixture.Response.Status)
	}
	shutdownBody := decodeBody(t, readAll(t, shutdownResp))
	if shutdownBody["accepted"] != true {
		t.Fatalf("unexpected shutdown response: %#v", shutdownBody)
	}

	statusAfterReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/system/status", nil)
	if err != nil {
		t.Fatalf("create post-shutdown system status request: %v", err)
	}
	statusAfterReq.Header.Set("Authorization", "Bearer "+token)
	statusAfterResp, err := server.Client().Do(statusAfterReq)
	if err != nil {
		t.Fatalf("perform post-shutdown system status request: %v", err)
	}
	defer func(release func() error) { _ = release() }(statusAfterResp.Body.Close)
	statusAfterBody := decodeBody(t, readAll(t, statusAfterResp))
	if statusAfterBody["status"] != "shutting_down" {
		t.Fatalf("unexpected post-shutdown status: %#v", statusAfterBody["status"])
	}
}

func TestLauncherStatusAndShutdownHandlers(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	statusFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.launcher-status.yaml"))
	statusReq, err := http.NewRequest(statusFixture.Request.Method, server.URL+statusFixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create launcher status request: %v", err)
	}
	statusReq.Header.Set("X-Raylea-Launcher-Control", testutil.TestLauncherControlToken)
	statusResp, err := server.Client().Do(statusReq)
	if err != nil {
		t.Fatalf("perform launcher status request: %v", err)
	}
	defer func(release func() error) { _ = release() }(statusResp.Body.Close)
	if statusResp.StatusCode != statusFixture.Response.Status {
		t.Fatalf("unexpected launcher status code: got %d want %d", statusResp.StatusCode, statusFixture.Response.Status)
	}
	statusBody := decodeBody(t, readAll(t, statusResp))
	if statusBody["status"] != "running" {
		t.Fatalf("unexpected launcher status: %#v", statusBody["status"])
	}
	if _, ok := statusBody["adapters"].([]any); !ok {
		t.Fatalf("expected adapters array, got %#v", statusBody["adapters"])
	}
	if _, ok := statusBody["active_plugins"].(float64); !ok {
		t.Fatalf("expected active_plugins number, got %#v", statusBody["active_plugins"])
	}
	if _, ok := statusBody["running_plugins"].(float64); !ok {
		t.Fatalf("expected running_plugins number, got %#v", statusBody["running_plugins"])
	}
	if _, ok := statusBody["failed_plugins"].(float64); !ok {
		t.Fatalf("expected failed_plugins number, got %#v", statusBody["failed_plugins"])
	}
	if _, ok := statusBody["db_schema_version"].(string); !ok {
		t.Fatalf("expected db_schema_version string, got %#v", statusBody["db_schema_version"])
	}
	if _, ok := statusBody["uptime_seconds"].(float64); !ok {
		t.Fatalf("expected uptime_seconds number, got %#v", statusBody["uptime_seconds"])
	}

	shutdownFixture := loadWebAPIFixtureDocument(t, testutil.RepoPath(t, "fixtures", "web-api", "ok.launcher-shutdown.yaml"))
	shutdownReq, err := http.NewRequest(shutdownFixture.Request.Method, server.URL+shutdownFixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create launcher shutdown request: %v", err)
	}
	shutdownReq.Header.Set("X-Raylea-Launcher-Control", testutil.TestLauncherControlToken)
	shutdownResp, err := server.Client().Do(shutdownReq)
	if err != nil {
		t.Fatalf("perform launcher shutdown request: %v", err)
	}
	defer func(release func() error) { _ = release() }(shutdownResp.Body.Close)
	if shutdownResp.StatusCode != shutdownFixture.Response.Status {
		t.Fatalf("unexpected launcher shutdown status: got %d want %d", shutdownResp.StatusCode, shutdownFixture.Response.Status)
	}
	shutdownBody := decodeBody(t, readAll(t, shutdownResp))
	if shutdownBody["accepted"] != true {
		t.Fatalf("unexpected launcher shutdown response: %#v", shutdownBody)
	}

	statusAfterReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/launcher/status", nil)
	if err != nil {
		t.Fatalf("create post-shutdown launcher status request: %v", err)
	}
	statusAfterReq.Header.Set("X-Raylea-Launcher-Control", testutil.TestLauncherControlToken)
	statusAfterResp, err := server.Client().Do(statusAfterReq)
	if err != nil {
		t.Fatalf("perform post-shutdown launcher status request: %v", err)
	}
	defer func(release func() error) { _ = release() }(statusAfterResp.Body.Close)
	statusAfterBody := decodeBody(t, readAll(t, statusAfterResp))
	if statusAfterBody["status"] != "shutting_down" {
		t.Fatalf("unexpected post-shutdown launcher status: %#v", statusAfterBody["status"])
	}
}

func TestLauncherHandlersRejectForwardedHeadersAndOldTokenRoutesAreGone(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "status", method: http.MethodGet, path: "/api/launcher/status"},
		{name: "shutdown", method: http.MethodPost, path: "/api/launcher/shutdown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, server.URL+tc.path, nil)
			if err != nil {
				t.Fatalf("create forwarded request: %v", err)
			}
			req.Header.Set("X-Forwarded-For", "198.51.100.9")

			resp, err := server.Client().Do(req)
			if err != nil {
				t.Fatalf("perform forwarded request: %v", err)
			}
			defer func(release func() error) { _ = release() }(resp.Body.Close)
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("unexpected forwarded status: got %d want 403", resp.StatusCode)
			}
			assertErrorEnvelopeMatchesFixture(t, decodeBody(t, readAll(t, resp)), map[string]any{
				"error": map[string]any{
					"code":        "permission.denied",
					"message":     "当前用户无权执行该操作",
					"message_key": "errors.permission.denied",
					"request_id":  "fixture_request_id_placeholder",
				},
			}, "permission.denied")
		})
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/api/session/launcher-token"},
		{method: http.MethodPost, path: "/api/session/launcher-admission"},
	} {
		req, err := http.NewRequest(tc.method, server.URL+tc.path, nil)
		if err != nil {
			t.Fatalf("create old launcher route request: %v", err)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatalf("perform old launcher route request: %v", err)
		}
		defer func(release func() error) { _ = release() }(resp.Body.Close)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("old launcher route %s returned %d, want 404", tc.path, resp.StatusCode)
		}
	}
}



func TestProtocolSnapshotHandler(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	snapshotReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/adapters", nil)
	if err != nil {
		t.Fatalf("create protocol snapshot request: %v", err)
	}
	snapshotReq.Header.Set("Authorization", "Bearer "+token)
	snapshotResp, err := server.Client().Do(snapshotReq)
	if err != nil {
		t.Fatalf("perform protocol snapshot request: %v", err)
	}
	defer func(release func() error) { _ = release() }(snapshotResp.Body.Close)
	if snapshotResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected protocol snapshot status: got %d want 200", snapshotResp.StatusCode)
	}
	adaptersBody := decodeBody(t, readAll(t, snapshotResp))
	snapshotBody := oneBotSnapshotForAdapter(t, adaptersBody, "onebot11")
	if snapshotBody["protocol"] != "onebot11" {
		t.Fatalf("unexpected protocol snapshot body: %#v", snapshotBody)
	}
	if _, ok := snapshotBody["transport_status"].([]any); !ok {
		t.Fatalf("expected transport_status array, got %#v", snapshotBody["transport_status"])
	}
}

func TestProtocolCompatibilityHandler(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/protocols/onebot11/compatibility", nil)
	if err != nil {
		t.Fatalf("create protocol compatibility request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform protocol compatibility request: %v", err)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected protocol compatibility status: got %d want 200", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	if body["protocol"] != "onebot11" {
		t.Fatalf("unexpected protocol compatibility body: %#v", body)
	}
	categories, ok := body["categories"].([]any)
	if !ok || len(categories) == 0 {
		t.Fatalf("expected compatibility categories, got %#v", body["categories"])
	}
}

func TestGovernanceBlacklistHandler(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	repo := permissionsqlite.NewAccessListRepository(application.Storage().Read, application.Storage().Write, permission.ListBlacklist)
	if err := repo.Add(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user", "10001", "反复触发垃圾消息"); err != nil {
		t.Fatalf("seed user blacklist entry: %v", err)
	}
	if err := repo.Add(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "group", "20002", "风险群已封禁"); err != nil {
		t.Fatalf("seed group blacklist entry: %v", err)
	}

	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/governance/blacklist", nil)
	if err != nil {
		t.Fatalf("create governance blacklist request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform governance blacklist request: %v", err)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected governance blacklist status: got %d want 200", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	userEntries, ok := body["user_entries"].([]any)
	if !ok || len(userEntries) != 1 {
		t.Fatalf("unexpected user_entries: %#v", body["user_entries"])
	}
	groupEntries, ok := body["group_entries"].([]any)
	if !ok || len(groupEntries) != 1 {
		t.Fatalf("unexpected group_entries: %#v", body["group_entries"])
	}

	userEntry := userEntries[0].(map[string]any)
	if userEntry["entry_type"] != "user" || userEntry["target_id"] != "10001" || userEntry["reason"] != "反复触发垃圾消息" {
		t.Fatalf("unexpected user entry: %#v", userEntry)
	}
	if _, err := time.Parse(time.RFC3339, userEntry["created_at"].(string)); err != nil {
		t.Fatalf("unexpected user created_at: %v", err)
	}

	groupEntry := groupEntries[0].(map[string]any)
	if groupEntry["entry_type"] != "group" || groupEntry["target_id"] != "20002" || groupEntry["reason"] != "风险群已封禁" {
		t.Fatalf("unexpected group entry: %#v", groupEntry)
	}
	if _, err := time.Parse(time.RFC3339, groupEntry["created_at"].(string)); err != nil {
		t.Fatalf("unexpected group created_at: %v", err)
	}
}

func TestGovernanceBlacklistWriteHandlers(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	repo := permissionsqlite.NewAccessListRepository(application.Storage().Read, application.Storage().Write, permission.ListBlacklist)
	if err := repo.Add(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user", "10001", "旧原因"); err != nil {
		t.Fatalf("seed blacklist entry: %v", err)
	}
	seeded, err := repo.Get(context.Background(), chatevent.IdentityScope{Kind: "global", SourceProtocol: "onebot11"}, "user", "10001")
	if err != nil {
		t.Fatalf("get seeded blacklist entry: %v", err)
	}

	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	upsertReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/governance/blacklist/entries", strings.NewReader(`{
  "entry_type": "user",
  "target_id": "10001",
  "reason": "新原因",
  "scope": {
    "kind":"global",
    "source_protocol": "onebot11",
    "source_adapter": "",
    "bot_id": ""
  }
}`))
	if err != nil {
		t.Fatalf("create blacklist upsert request: %v", err)
	}
	upsertReq.Header.Set("Authorization", "Bearer "+token)
	upsertReq.Header.Set("Content-Type", "application/json")

	upsertResp, err := server.Client().Do(upsertReq)
	if err != nil {
		t.Fatalf("perform blacklist upsert request: %v", err)
	}
	defer func(release func() error) { _ = release() }(upsertResp.Body.Close)
	if upsertResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected blacklist upsert status: got %d want 200", upsertResp.StatusCode)
	}

	upsertBody := decodeBody(t, readAll(t, upsertResp))
	if upsertBody["reason"] != "新原因" {
		t.Fatalf("unexpected blacklist upsert body: %#v", upsertBody)
	}
	if upsertBody["created_at"] != seeded.CreatedAt {
		t.Fatalf("created_at = %#v, want %q", upsertBody["created_at"], seeded.CreatedAt)
	}

	invalidReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/governance/blacklist/entries", strings.NewReader(`{
  "entry_type": "user",
  "target_id": "10001",
  "reason": "",
  "scope": {
    "kind":"global",
    "source_protocol": "onebot11",
    "source_adapter": "",
    "bot_id": ""
  }
}`))
	if err != nil {
		t.Fatalf("create invalid blacklist upsert request: %v", err)
	}
	invalidReq.Header.Set("Authorization", "Bearer "+token)
	invalidReq.Header.Set("Content-Type", "application/json")

	invalidResp, err := server.Client().Do(invalidReq)
	if err != nil {
		t.Fatalf("perform invalid blacklist upsert request: %v", err)
	}
	defer func(release func() error) { _ = release() }(invalidResp.Body.Close)
	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected invalid blacklist upsert status: got %d want 400", invalidResp.StatusCode)
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/governance/blacklist/entries/user/10001?kind=global&source_protocol=onebot11", nil)
	if err != nil {
		t.Fatalf("create blacklist delete request: %v", err)
	}
	deleteReq.Header.Set("Authorization", "Bearer "+token)

	deleteResp, err := server.Client().Do(deleteReq)
	if err != nil {
		t.Fatalf("perform blacklist delete request: %v", err)
	}
	defer func(release func() error) { _ = release() }(deleteResp.Body.Close)
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected blacklist delete status: got %d want 204", deleteResp.StatusCode)
	}

	missingReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/governance/blacklist/entries/user/10001?kind=global&source_protocol=onebot11", nil)
	if err != nil {
		t.Fatalf("create missing blacklist delete request: %v", err)
	}
	missingReq.Header.Set("Authorization", "Bearer "+token)

	missingResp, err := server.Client().Do(missingReq)
	if err != nil {
		t.Fatalf("perform missing blacklist delete request: %v", err)
	}
	defer func(release func() error) { _ = release() }(missingResp.Body.Close)
	if missingResp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected missing blacklist delete status: got %d want 404", missingResp.StatusCode)
	}

	missingBody := decodeBody(t, readAll(t, missingResp))
	errorBody, ok := missingBody["error"].(map[string]any)
	if !ok || errorBody["code"] != "platform.resource_not_found" {
		t.Fatalf("unexpected missing blacklist delete body: %#v", missingBody)
	}
}
