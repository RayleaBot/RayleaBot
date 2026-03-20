package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
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

	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	afterFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-status.yaml"))
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
	server := httptest.NewServer(application.Handler())
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
	defer response.Body.Close()
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
	defer protectedResp.Body.Close()
	if protectedResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected protected status after logout: got %d want 401", protectedResp.StatusCode)
	}
}

func TestLauncherTokenIssuanceReturnsOpaqueToken(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.session-launcher-token.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create launcher-token request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform launcher-token request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fixture.Response.Status {
		t.Fatalf("unexpected launcher-token status: got %d want %d", response.StatusCode, fixture.Response.Status)
	}

	body := decodeBody(t, readAll(t, response))
	value, ok := body["launcher_token"].(string)
	if !ok || value == "" {
		t.Fatalf("expected non-empty launcher_token, got %#v", body["launcher_token"])
	}
	if len(body) != 1 {
		t.Fatalf("unexpected launcher-token body shape: %#v", body)
	}
	if strings.Contains(value, token) {
		t.Fatalf("launcher token leaked session token content: %q", value)
	}
}

func TestSystemStatusAndShutdownHandlers(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
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
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected system status code: got %d want 200", statusResp.StatusCode)
	}
	statusBody := decodeBody(t, readAll(t, statusResp))
	if statusBody["status"] != "running" {
		t.Fatalf("unexpected system status: %#v", statusBody["status"])
	}
	if _, ok := statusBody["adapter_state"].(string); !ok {
		t.Fatalf("expected adapter_state string, got %#v", statusBody["adapter_state"])
	}
	if _, ok := statusBody["active_plugins"].(float64); !ok {
		t.Fatalf("expected active_plugins number, got %#v", statusBody["active_plugins"])
	}
	if _, ok := statusBody["uptime_seconds"].(float64); !ok {
		t.Fatalf("expected uptime_seconds number, got %#v", statusBody["uptime_seconds"])
	}

	shutdownFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.system-shutdown.yaml"))
	shutdownReq, err := http.NewRequest(http.MethodPost, server.URL+shutdownFixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create system shutdown request: %v", err)
	}
	shutdownReq.Header.Set("Authorization", "Bearer "+token)
	shutdownResp, err := server.Client().Do(shutdownReq)
	if err != nil {
		t.Fatalf("perform system shutdown request: %v", err)
	}
	defer shutdownResp.Body.Close()
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
	defer statusAfterResp.Body.Close()
	statusAfterBody := decodeBody(t, readAll(t, statusAfterResp))
	if statusAfterBody["status"] != "shutting_down" {
		t.Fatalf("unexpected post-shutdown status: %#v", statusAfterBody["status"])
	}
}
