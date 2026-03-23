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
	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	fixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.session-launcher-token.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	setupReq, err := http.NewRequest(setupFixture.Request.Method, server.URL+setupFixture.Request.Path, strings.NewReader(`{"identifier":"admin","secret":"fixture-only-secret"}`))
	if err != nil {
		t.Fatalf("create setup request: %v", err)
	}
	setupReq.Header.Set("Content-Type", "application/json")
	setupResp, err := server.Client().Do(setupReq)
	if err != nil {
		t.Fatalf("perform setup request: %v", err)
	}
	defer setupResp.Body.Close()
	if setupResp.StatusCode != setupFixture.Response.Status {
		t.Fatalf("unexpected setup status: got %d want %d", setupResp.StatusCode, setupFixture.Response.Status)
	}

	request, err := http.NewRequest(http.MethodPost, server.URL+fixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create launcher-token request: %v", err)
	}

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
}

func TestLauncherAdmissionConsumesTokenAndReturnsSession(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	setupFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.setup-admin.yaml"))
	tokenFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.session-launcher-token.yaml"))
	admissionFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.session-launcher-admission.yaml"))
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	setupReq, err := http.NewRequest(setupFixture.Request.Method, server.URL+setupFixture.Request.Path, strings.NewReader(`{"identifier":"admin","secret":"fixture-only-secret"}`))
	if err != nil {
		t.Fatalf("create setup request: %v", err)
	}
	setupReq.Header.Set("Content-Type", "application/json")
	setupResp, err := server.Client().Do(setupReq)
	if err != nil {
		t.Fatalf("perform setup request: %v", err)
	}
	defer setupResp.Body.Close()
	if setupResp.StatusCode != setupFixture.Response.Status {
		t.Fatalf("unexpected setup status: got %d want %d", setupResp.StatusCode, setupFixture.Response.Status)
	}

	issueReq, err := http.NewRequest(http.MethodPost, server.URL+tokenFixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create launcher-token request: %v", err)
	}
	issueResp, err := server.Client().Do(issueReq)
	if err != nil {
		t.Fatalf("perform launcher-token request: %v", err)
	}
	defer issueResp.Body.Close()
	if issueResp.StatusCode != tokenFixture.Response.Status {
		t.Fatalf("unexpected launcher-token status: got %d want %d", issueResp.StatusCode, tokenFixture.Response.Status)
	}
	launcherToken, ok := decodeBody(t, readAll(t, issueResp))["launcher_token"].(string)
	if !ok || launcherToken == "" {
		t.Fatalf("expected non-empty launcher_token")
	}

	admissionReq, err := http.NewRequest(admissionFixture.Request.Method, server.URL+admissionFixture.Request.Path, strings.NewReader(`{"launcher_token":"`+launcherToken+`"}`))
	if err != nil {
		t.Fatalf("create launcher-admission request: %v", err)
	}
	admissionReq.Header.Set("Content-Type", "application/json")
	admissionResp, err := server.Client().Do(admissionReq)
	if err != nil {
		t.Fatalf("perform launcher-admission request: %v", err)
	}
	defer admissionResp.Body.Close()
	if admissionResp.StatusCode != admissionFixture.Response.Status {
		t.Fatalf("unexpected launcher-admission status: got %d want %d", admissionResp.StatusCode, admissionFixture.Response.Status)
	}
	sessionToken, ok := decodeBody(t, readAll(t, admissionResp))["session_token"].(string)
	if !ok || sessionToken == "" {
		t.Fatalf("expected non-empty session_token from launcher admission")
	}

	protectedReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/system/status", nil)
	if err != nil {
		t.Fatalf("create protected request: %v", err)
	}
	protectedReq.Header.Set("Authorization", "Bearer "+sessionToken)
	protectedResp, err := server.Client().Do(protectedReq)
	if err != nil {
		t.Fatalf("perform protected request: %v", err)
	}
	defer protectedResp.Body.Close()
	if protectedResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected protected status: got %d want 200", protectedResp.StatusCode)
	}

	reuseReq, err := http.NewRequest(http.MethodPost, server.URL+admissionFixture.Request.Path, strings.NewReader(`{"launcher_token":"`+launcherToken+`"}`))
	if err != nil {
		t.Fatalf("create second launcher-admission request: %v", err)
	}
	reuseReq.Header.Set("Content-Type", "application/json")
	reuseResp, err := server.Client().Do(reuseReq)
	if err != nil {
		t.Fatalf("perform second launcher-admission request: %v", err)
	}
	defer reuseResp.Body.Close()
	if reuseResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected second launcher-admission status: got %d want 401", reuseResp.StatusCode)
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
