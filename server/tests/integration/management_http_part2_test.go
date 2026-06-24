package integration

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGovernanceWhitelistHandlers(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	entryRepo := permission.NewSQLiteWhitelistRepository(application.Storage().Read, application.Storage().Write)
	stateRepo := permission.NewSQLiteWhitelistStateRepository(application.Storage().Read, application.Storage().Write)

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	getReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/governance/whitelist", nil)
	if err != nil {
		t.Fatalf("create whitelist get request: %v", err)
	}
	getReq.Header.Set("Authorization", "Bearer "+token)

	getResp, err := server.Client().Do(getReq)
	if err != nil {
		t.Fatalf("perform whitelist get request: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected whitelist get status: got %d want 200", getResp.StatusCode)
	}

	initialBody := decodeBody(t, readAll(t, getResp))
	if initialBody["enabled"] != false {
		t.Fatalf("unexpected initial whitelist enabled: %#v", initialBody["enabled"])
	}

	upsertReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/governance/whitelist/entries", strings.NewReader(`{"entry_type":"user","target_id":"10001","reason":"值班账号"}`))
	if err != nil {
		t.Fatalf("create whitelist upsert request: %v", err)
	}
	upsertReq.Header.Set("Authorization", "Bearer "+token)
	upsertReq.Header.Set("Content-Type", "application/json")

	upsertResp, err := server.Client().Do(upsertReq)
	if err != nil {
		t.Fatalf("perform whitelist upsert request: %v", err)
	}
	defer upsertResp.Body.Close()
	if upsertResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected whitelist upsert status: got %d want 200", upsertResp.StatusCode)
	}
	upsertBody := decodeBody(t, readAll(t, upsertResp))
	if upsertBody["target_id"] != "10001" || upsertBody["reason"] != "值班账号" {
		t.Fatalf("unexpected whitelist upsert body: %#v", upsertBody)
	}

	groupReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/governance/whitelist/entries", strings.NewReader(`{"entry_type":"group","target_id":"20002","reason":"核心服务群"}`))
	if err != nil {
		t.Fatalf("create group whitelist upsert request: %v", err)
	}
	groupReq.Header.Set("Authorization", "Bearer "+token)
	groupReq.Header.Set("Content-Type", "application/json")

	groupResp, err := server.Client().Do(groupReq)
	if err != nil {
		t.Fatalf("perform group whitelist upsert request: %v", err)
	}
	defer groupResp.Body.Close()
	if groupResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected group whitelist upsert status: got %d want 200", groupResp.StatusCode)
	}

	enableReq, err := http.NewRequest(http.MethodPut, server.URL+"/api/governance/whitelist/state", strings.NewReader(`{"enabled":true}`))
	if err != nil {
		t.Fatalf("create whitelist state request: %v", err)
	}
	enableReq.Header.Set("Authorization", "Bearer "+token)
	enableReq.Header.Set("Content-Type", "application/json")

	enableResp, err := server.Client().Do(enableReq)
	if err != nil {
		t.Fatalf("perform whitelist state request: %v", err)
	}
	defer enableResp.Body.Close()
	if enableResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected whitelist state status: got %d want 200", enableResp.StatusCode)
	}
	enableBody := decodeBody(t, readAll(t, enableResp))
	if enableBody["enabled"] != true {
		t.Fatalf("unexpected whitelist state body: %#v", enableBody)
	}

	enabled, err := stateRepo.Enabled(context.Background())
	if err != nil {
		t.Fatalf("read whitelist state repo: %v", err)
	}
	if !enabled {
		t.Fatal("expected whitelist state repo to be enabled")
	}

	snapshotReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/governance/whitelist", nil)
	if err != nil {
		t.Fatalf("create enabled whitelist get request: %v", err)
	}
	snapshotReq.Header.Set("Authorization", "Bearer "+token)

	snapshotResp, err := server.Client().Do(snapshotReq)
	if err != nil {
		t.Fatalf("perform enabled whitelist get request: %v", err)
	}
	defer snapshotResp.Body.Close()
	if snapshotResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected enabled whitelist get status: got %d want 200", snapshotResp.StatusCode)
	}

	snapshotBody := decodeBody(t, readAll(t, snapshotResp))
	if snapshotBody["enabled"] != true {
		t.Fatalf("unexpected whitelist enabled snapshot: %#v", snapshotBody["enabled"])
	}
	if userEntries, ok := snapshotBody["user_entries"].([]any); !ok || len(userEntries) != 1 {
		t.Fatalf("unexpected whitelist user entries: %#v", snapshotBody["user_entries"])
	}
	if groupEntries, ok := snapshotBody["group_entries"].([]any); !ok || len(groupEntries) != 1 {
		t.Fatalf("unexpected whitelist group entries: %#v", snapshotBody["group_entries"])
	}

	invalidReq, err := http.NewRequest(http.MethodPut, server.URL+"/api/governance/whitelist/state", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("create invalid whitelist state request: %v", err)
	}
	invalidReq.Header.Set("Authorization", "Bearer "+token)
	invalidReq.Header.Set("Content-Type", "application/json")

	invalidResp, err := server.Client().Do(invalidReq)
	if err != nil {
		t.Fatalf("perform invalid whitelist state request: %v", err)
	}
	defer invalidResp.Body.Close()
	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected invalid whitelist state status: got %d want 400", invalidResp.StatusCode)
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/governance/whitelist/entries/group/20002", nil)
	if err != nil {
		t.Fatalf("create whitelist delete request: %v", err)
	}
	deleteReq.Header.Set("Authorization", "Bearer "+token)

	deleteResp, err := server.Client().Do(deleteReq)
	if err != nil {
		t.Fatalf("perform whitelist delete request: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected whitelist delete status: got %d want 204", deleteResp.StatusCode)
	}

	if _, err := entryRepo.Get(context.Background(), "group", "20002"); err != permission.ErrGovernanceEntryNotFound {
		t.Fatalf("group whitelist entry should be removed, got err=%v", err)
	}
}

func TestGovernanceCommandPolicyHandler(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	application.Plugins().Replace([]plugins.Snapshot{
		{
			PluginID:          "weather",
			Name:              "Weather",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name:       "weather",
				Aliases:    []string{"tq", "天气"},
				Permission: "group_admin",
			}},
		},
		{
			PluginID:          "hello-python",
			Name:              "Hello Python",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			RuntimeState:      "running",
			Commands: []plugins.Command{{
				Name:    "hello",
				Aliases: []string{"hi"},
			}},
		},
		{
			PluginID:          "disabled-plugin",
			Name:              "Disabled Plugin",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "disabled",
			RuntimeState:      "stopped",
			Commands: []plugins.Command{{
				Name: "skip-disabled",
			}},
		},
	})

	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/governance/command-policy", nil)
	if err != nil {
		t.Fatalf("create governance command-policy request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform governance command-policy request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected governance command-policy status: got %d want 200", response.StatusCode)
	}

	body := decodeBody(t, readAll(t, response))
	if body["default_level"] != "everyone" {
		t.Fatalf("unexpected default_level: %#v", body["default_level"])
	}
	cooldown, ok := body["cooldown"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected cooldown payload: %#v", body["cooldown"])
	}
	if cooldown["user_command_rate_limit"] != "10/60s" || cooldown["group_command_rate_limit"] != "30/60s" || cooldown["cooldown_reply"] != true {
		t.Fatalf("unexpected cooldown payload: %#v", cooldown)
	}

	commands, ok := body["commands"].([]any)
	if !ok || len(commands) != 2 {
		t.Fatalf("unexpected commands payload: %#v", body["commands"])
	}

	byPluginID := make(map[string]map[string]any, len(commands))
	for _, item := range commands {
		entry := item.(map[string]any)
		byPluginID[entry["plugin_id"].(string)] = entry
	}

	weather := byPluginID["weather"]
	if weather["plugin_name"] != "Weather" || weather["command"] != "weather" {
		t.Fatalf("unexpected weather command policy entry: %#v", weather)
	}
	if !reflect.DeepEqual(weather["aliases"], []any{"tq", "天气"}) {
		t.Fatalf("unexpected weather aliases: %#v", weather["aliases"])
	}
	if weather["declared_permission"] != "group_admin" || weather["effective_permission"] != "group_admin" || weather["permission_source"] != "declared" {
		t.Fatalf("unexpected weather permission policy: %#v", weather)
	}

	hello := byPluginID["hello-python"]
	if hello["plugin_name"] != "Hello Python" || hello["command"] != "hello" {
		t.Fatalf("unexpected hello command policy entry: %#v", hello)
	}
	if !reflect.DeepEqual(hello["aliases"], []any{"hi"}) {
		t.Fatalf("unexpected hello aliases: %#v", hello["aliases"])
	}
	if hello["declared_permission"] != nil || hello["effective_permission"] != "everyone" || hello["permission_source"] != "default_level" {
		t.Fatalf("unexpected hello permission policy: %#v", hello)
	}
}

func TestSystemBackupAcceptsTaskAndCreatesArchive(t *testing.T) {
	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.system-backup-accepted.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create system backup request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform system backup request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected system backup status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		t.Fatalf("unexpected system backup body: %#v", body)
	}

	snapshot := waitForTaskStatus(t, application.Tasks(), taskID, "succeeded")
	if snapshot.TaskType != "backup.create" {
		t.Fatalf("unexpected backup task type: got %q want %q", snapshot.TaskType, "backup.create")
	}
	if snapshot.Result == nil {
		t.Fatalf("expected backup task result, got %#v", snapshot)
	}

	archivePath, ok := snapshot.Result.Details["archive_path"].(string)
	if !ok || archivePath == "" {
		t.Fatalf("expected backup archive path in result details, got %#v", snapshot.Result.Details)
	}
	t.Cleanup(func() {
		_ = os.Remove(archivePath)
	})

	info, err := os.Stat(archivePath)
	if err != nil {
		t.Fatalf("stat backup archive: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty backup archive: %s", archivePath)
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open backup archive: %v", err)
	}
	defer reader.Close()

	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
	}
	if !entries["backup-manifest.json"] {
		t.Fatalf("backup archive missing backup-manifest.json: %#v", entries)
	}
}

func TestSystemDiagnosticsExportReturnsZipBundle(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.system-diagnostics-export.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodGet, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create diagnostics export request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform diagnostics export request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected diagnostics export status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}
	if got := response.Header.Get("Content-Type"); got != fixture.Response.Headers["Content-Type"] {
		t.Fatalf("unexpected diagnostics content-type: got %q want %q", got, fixture.Response.Headers["Content-Type"])
	}
	if got := response.Header.Get("Content-Disposition"); got != fixture.Response.Headers["Content-Disposition"] {
		t.Fatalf("unexpected diagnostics content-disposition: got %q want %q", got, fixture.Response.Headers["Content-Disposition"])
	}

	payload := readAll(t, response)
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("open diagnostics archive: %v", err)
	}

	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
	}

	for _, required := range []string{"system-status.json", "readiness.json", "doctor.json", "plugins.json", "config-summary.json", "recent-logs.json"} {
		if !entries[required] {
			t.Fatalf("diagnostics archive missing %s: %#v", required, entries)
		}
	}

	for _, file := range reader.File {
		if file.Name != "doctor.json" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open doctor.json: %v", err)
		}
		defer rc.Close()

		var body map[string]any
		if err := json.NewDecoder(rc).Decode(&body); err != nil {
			t.Fatalf("decode doctor.json: %v", err)
		}

		issues, ok := body["issues"].([]any)
		if !ok || len(issues) == 0 {
			t.Fatalf("doctor.json must contain issues: %#v", body)
		}

		first, ok := issues[0].(map[string]any)
		if !ok {
			t.Fatalf("doctor.json first issue malformed: %#v", issues[0])
		}
		for _, key := range []string{"code", "severity", "summary", "remediation"} {
			if _, ok := first[key]; !ok {
				t.Fatalf("doctor.json issue missing %s: %#v", key, first)
			}
		}
		break
	}
}
