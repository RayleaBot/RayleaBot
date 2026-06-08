package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	internalapp "github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
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

func TestLauncherStatusAndShutdownHandlers(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	statusFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.launcher-status.yaml"))
	statusReq, err := http.NewRequest(statusFixture.Request.Method, server.URL+statusFixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create launcher status request: %v", err)
	}
	statusResp, err := server.Client().Do(statusReq)
	if err != nil {
		t.Fatalf("perform launcher status request: %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != statusFixture.Response.Status {
		t.Fatalf("unexpected launcher status code: got %d want %d", statusResp.StatusCode, statusFixture.Response.Status)
	}
	statusBody := decodeBody(t, readAll(t, statusResp))
	if statusBody["status"] != "running" {
		t.Fatalf("unexpected launcher status: %#v", statusBody["status"])
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

	shutdownFixture := loadWebAPIFixtureDocument(t, filepath.Join("..", "fixtures", "web-api", "ok.launcher-shutdown.yaml"))
	shutdownReq, err := http.NewRequest(shutdownFixture.Request.Method, server.URL+shutdownFixture.Request.Path, nil)
	if err != nil {
		t.Fatalf("create launcher shutdown request: %v", err)
	}
	shutdownResp, err := server.Client().Do(shutdownReq)
	if err != nil {
		t.Fatalf("perform launcher shutdown request: %v", err)
	}
	defer shutdownResp.Body.Close()
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
	statusAfterResp, err := server.Client().Do(statusAfterReq)
	if err != nil {
		t.Fatalf("perform post-shutdown launcher status request: %v", err)
	}
	defer statusAfterResp.Body.Close()
	statusAfterBody := decodeBody(t, readAll(t, statusAfterResp))
	if statusAfterBody["status"] != "shutting_down" {
		t.Fatalf("unexpected post-shutdown launcher status: %#v", statusAfterBody["status"])
	}
}

func TestLauncherHandlersRejectForwardedHeadersAndOldTokenRoutesAreGone(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	server := httptest.NewServer(application.Handler())
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
			defer resp.Body.Close()
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
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("old launcher route %s returned %d, want 404", tc.path, resp.StatusCode)
		}
	}
}

func TestThirdPartyAccountAndBilibiliSourceHandlers(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithOptions(t, nil, func(options *internalapp.Options, _ string) {
		options.BilibiliHTTPTransport = managementBilibiliTransport(t)
		options.BilibiliClock = func() time.Time { return time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC) }
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	doRequest := func(method, path, body string) (*http.Response, []byte) {
		t.Helper()
		request, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatalf("create %s %s request: %v", method, path, err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatalf("perform %s %s request: %v", method, path, err)
		}
		payload := readAll(t, response)
		return response, payload
	}

	cookie := "SESSDATA=fixture-secret; bili_jct=fixture-csrf;"
	upsertResp, upsertPayload := doRequest(http.MethodPut, "/api/third-party/accounts/bilibili/primary", `{"label":"主账号","enabled":true,"cookie":"`+cookie+`"}`)
	defer upsertResp.Body.Close()
	if upsertResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected third-party account upsert status: got %d want 200 body=%s", upsertResp.StatusCode, string(upsertPayload))
	}
	if strings.Contains(string(upsertPayload), "fixture-secret") || strings.Contains(string(upsertPayload), "fixture-csrf") {
		t.Fatalf("third-party account upsert response leaked cookie: %s", string(upsertPayload))
	}
	upsertBody := decodeBody(t, upsertPayload)
	account, ok := upsertBody["account"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected third-party account upsert body: %#v", upsertBody)
	}
	if account["platform"] != "bilibili" || account["account_id"] != "primary" || account["label"] != "主账号" || account["enabled"] != true || account["configured"] != true {
		t.Fatalf("unexpected third-party account summary: %#v", account)
	}
	profile, ok := account["profile"].(map[string]any)
	if !ok || profile["uid"] != "123456" || profile["nickname"] != "主账号昵称" || profile["avatar_url"] != "https://i0.hdslb.com/bfs/face/raylea.jpg" {
		t.Fatalf("unexpected third-party account profile: %#v", account["profile"])
	}
	credential, ok := account["credential"].(map[string]any)
	if !ok || credential["state"] != "valid" || credential["checked_at"] != "2026-06-08T08:00:00Z" || credential["last_error"] != "" {
		t.Fatalf("unexpected third-party credential: %#v", account["credential"])
	}
	polling, ok := account["polling"].(map[string]any)
	if !ok || polling["enabled"] != true || polling["last_used_at"] != nil {
		t.Fatalf("unexpected third-party polling status: %#v", account["polling"])
	}
	updatedAt, ok := account["updated_at"].(string)
	if !ok {
		t.Fatalf("expected third-party account updated_at string, got %#v", account["updated_at"])
	}
	if _, err := time.Parse(time.RFC3339, updatedAt); err != nil {
		t.Fatalf("unexpected third-party account updated_at: %v", err)
	}

	listResp, listPayload := doRequest(http.MethodGet, "/api/third-party/accounts", "")
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected third-party account list status: got %d want 200 body=%s", listResp.StatusCode, string(listPayload))
	}
	if strings.Contains(string(listPayload), "fixture-secret") || strings.Contains(string(listPayload), "fixture-csrf") {
		t.Fatalf("third-party account list response leaked cookie: %s", string(listPayload))
	}
	listBody := decodeBody(t, listPayload)
	items, ok := listBody["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected third-party account list body: %#v", listBody)
	}
	listAccount := items[0].(map[string]any)
	if listAccount["platform"] != "bilibili" || listAccount["account_id"] != "primary" || listAccount["configured"] != true {
		t.Fatalf("unexpected third-party account list item: %#v", listAccount)
	}
	listProfile, ok := listAccount["profile"].(map[string]any)
	if !ok || listProfile["nickname"] != "主账号昵称" || listProfile["uid"] != "123456" {
		t.Fatalf("unexpected third-party account list profile: %#v", listAccount["profile"])
	}

	statusResp, statusPayload := doRequest(http.MethodGet, "/api/bilibili/source/status", "")
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected bilibili source status code: got %d want 200 body=%s", statusResp.StatusCode, string(statusPayload))
	}
	statusBody := decodeBody(t, statusPayload)
	if statusBody["status"] != "idle" || statusBody["summary"] != "Bilibili 事件源等待订阅" {
		t.Fatalf("unexpected bilibili source status body: %#v", statusBody)
	}
	live, ok := statusBody["live"].(map[string]any)
	if !ok || live["watched_rooms"] == nil || live["fallback_polling"] == nil {
		t.Fatalf("unexpected bilibili live status: %#v", statusBody["live"])
	}
	dynamic, ok := statusBody["dynamic"].(map[string]any)
	if !ok || dynamic["interval_seconds"] != float64(10) || dynamic["auto_follow"] != true {
		t.Fatalf("unexpected bilibili dynamic status: %#v", statusBody["dynamic"])
	}
	statusAccounts, ok := statusBody["accounts"].([]any)
	if !ok || len(statusAccounts) != 1 {
		t.Fatalf("unexpected bilibili source accounts: %#v", statusBody["accounts"])
	}
	statusAccount := statusAccounts[0].(map[string]any)
	if statusAccount["platform"] != "bilibili" || statusAccount["configured"] != true {
		t.Fatalf("unexpected bilibili source account summary: %#v", statusAccount)
	}

	monitorsResp, monitorsPayload := doRequest(http.MethodGet, "/api/third-party/monitors?platform=bilibili", "")
	defer monitorsResp.Body.Close()
	if monitorsResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected third-party monitors code: got %d want 200 body=%s", monitorsResp.StatusCode, string(monitorsPayload))
	}
	monitorsBody := decodeBody(t, monitorsPayload)
	if monitorsBody["platform"] != "bilibili" {
		t.Fatalf("unexpected third-party monitors platform: %#v", monitorsBody)
	}
	monitorItems, ok := monitorsBody["items"].([]any)
	if !ok || len(monitorItems) != 0 {
		t.Fatalf("unexpected third-party monitor items: %#v", monitorsBody)
	}
	invalidMonitorsResp, invalidMonitorsPayload := doRequest(http.MethodGet, "/api/third-party/monitors?platform=twitter", "")
	defer invalidMonitorsResp.Body.Close()
	if invalidMonitorsResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected invalid third-party monitors code: got %d want 400 body=%s", invalidMonitorsResp.StatusCode, string(invalidMonitorsPayload))
	}

	restartResp, restartPayload := doRequest(http.MethodPost, "/api/bilibili/source/restart", "")
	defer restartResp.Body.Close()
	if restartResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected bilibili source restart code: got %d want 200 body=%s", restartResp.StatusCode, string(restartPayload))
	}
	restartBody := decodeBody(t, restartPayload)
	if restartBody["accepted"] != true {
		t.Fatalf("unexpected bilibili source restart body: %#v", restartBody)
	}
	if _, ok := restartBody["status"].(map[string]any); !ok {
		t.Fatalf("expected bilibili source restart status snapshot, got %#v", restartBody["status"])
	}

	qrCreateResp, qrCreatePayload := doRequest(http.MethodPost, "/api/bilibili/login/qrcode", "")
	defer qrCreateResp.Body.Close()
	if qrCreateResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected bilibili qr create code: got %d want 200 body=%s", qrCreateResp.StatusCode, string(qrCreatePayload))
	}
	qrCreateBody := decodeBody(t, qrCreatePayload)
	if qrCreateBody["state"] != "pending_scan" || qrCreateBody["qrcode_url"] != "https://passport.bilibili.com/h5-app/passport/login/scan?navhide=1&qrcode_key=fixture-key" {
		t.Fatalf("unexpected bilibili qr create body: %#v", qrCreateBody)
	}
	loginID, ok := qrCreateBody["login_id"].(string)
	if !ok || !strings.HasPrefix(loginID, "qr_") {
		t.Fatalf("unexpected bilibili qr login id: %#v", qrCreateBody["login_id"])
	}

	qrPendingResp, qrPendingPayload := doRequest(http.MethodGet, "/api/bilibili/login/qrcode/"+loginID, "")
	defer qrPendingResp.Body.Close()
	if qrPendingResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected bilibili qr pending code: got %d want 200 body=%s", qrPendingResp.StatusCode, string(qrPendingPayload))
	}
	qrPendingBody := decodeBody(t, qrPendingPayload)
	if qrPendingBody["state"] != "pending_confirm" || qrPendingBody["cookie"] != nil || qrPendingBody["account"] != nil {
		t.Fatalf("unexpected bilibili qr pending body: %#v", qrPendingBody)
	}

	qrSucceededResp, qrSucceededPayload := doRequest(http.MethodGet, "/api/bilibili/login/qrcode/"+loginID, "")
	defer qrSucceededResp.Body.Close()
	if qrSucceededResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected bilibili qr succeeded code: got %d want 200 body=%s", qrSucceededResp.StatusCode, string(qrSucceededPayload))
	}
	if !strings.Contains(string(qrSucceededPayload), "fixture-secret") {
		t.Fatalf("expected qr poll success to return cookie, got %s", string(qrSucceededPayload))
	}
	qrSucceededBody := decodeBody(t, qrSucceededPayload)
	qrAccount, ok := qrSucceededBody["account"].(map[string]any)
	if qrSucceededBody["state"] != "succeeded" || !ok || qrAccount["uid"] != "123456" || qrAccount["nickname"] != "主账号昵称" {
		t.Fatalf("unexpected bilibili qr succeeded body: %#v", qrSucceededBody)
	}

	deleteResp, deletePayload := doRequest(http.MethodDelete, "/api/third-party/accounts/bilibili/primary", "")
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected third-party account delete status: got %d want 204 body=%s", deleteResp.StatusCode, string(deletePayload))
	}

	emptyResp, emptyPayload := doRequest(http.MethodGet, "/api/third-party/accounts", "")
	defer emptyResp.Body.Close()
	if emptyResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected empty third-party account list status: got %d want 200 body=%s", emptyResp.StatusCode, string(emptyPayload))
	}
	emptyBody := decodeBody(t, emptyPayload)
	emptyItems, ok := emptyBody["items"].([]any)
	if !ok || len(emptyItems) != 0 {
		t.Fatalf("expected empty third-party account list, got %#v", emptyBody)
	}
}

func managementBilibiliTransport(t *testing.T) http.RoundTripper {
	t.Helper()

	qrPollCount := 0
	return managementRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.URL.Host == "api.bilibili.com" && request.URL.Path == "/x/web-interface/nav":
			if !strings.Contains(request.Header.Get("Cookie"), "SESSDATA=") {
				return nil, fmt.Errorf("expected nav cookie header, got %q", request.Header.Get("Cookie"))
			}
			return managementJSONResponse(`{
				"code": 0,
				"data": {
					"isLogin": true,
					"mid": 123456,
					"uname": "主账号昵称",
					"face": "//i0.hdslb.com/bfs/face/raylea.jpg"
				}
			}`), nil
		case request.URL.Host == "passport.bilibili.com" && request.URL.Path == "/x/passport-login/web/qrcode/generate":
			return managementJSONResponse(`{
				"code": 0,
				"data": {
					"url": "https://passport.bilibili.com/h5-app/passport/login/scan?navhide=1&qrcode_key=fixture-key",
					"qrcode_key": "fixture-key"
				}
			}`), nil
		case request.URL.Host == "passport.bilibili.com" && request.URL.Path == "/x/passport-login/web/qrcode/poll":
			if request.URL.Query().Get("qrcode_key") != "fixture-key" {
				return nil, fmt.Errorf("unexpected qrcode_key: %s", request.URL.RawQuery)
			}
			qrPollCount += 1
			if qrPollCount == 1 {
				return managementJSONResponse(`{"code":0,"data":{"code":86090,"message":"waiting"}}`), nil
			}
			return managementJSONResponse(`{
				"code": 0,
				"data": {
					"code": 0,
					"url": "https://passport.bilibili.com/login?SESSDATA=fixture-secret&bili_jct=fixture-csrf&DedeUserID=123456",
					"refresh_token": "fixture-refresh-token"
				}
			}`), nil
		default:
			return nil, fmt.Errorf("unexpected bilibili request: %s %s", request.Method, request.URL.String())
		}
	})
}

type managementRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn managementRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func managementJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestProtocolSnapshotHandler(t *testing.T) {
	t.Parallel()

	application := newTestApp(t, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := httptest.NewServer(application.Handler())
	defer server.Close()

	snapshotReq, err := http.NewRequest(http.MethodGet, server.URL+"/api/protocols/onebot11", nil)
	if err != nil {
		t.Fatalf("create protocol snapshot request: %v", err)
	}
	snapshotReq.Header.Set("Authorization", "Bearer "+token)
	snapshotResp, err := server.Client().Do(snapshotReq)
	if err != nil {
		t.Fatalf("perform protocol snapshot request: %v", err)
	}
	defer snapshotResp.Body.Close()
	if snapshotResp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected protocol snapshot status: got %d want 200", snapshotResp.StatusCode)
	}
	snapshotBody := decodeBody(t, readAll(t, snapshotResp))
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
	server := httptest.NewServer(application.Handler())
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
	defer response.Body.Close()
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
	repo := permission.NewSQLiteBlacklistRepository(application.Storage().Read, application.Storage().Write)
	if err := repo.Add(context.Background(), "user", "10001", "反复触发垃圾消息"); err != nil {
		t.Fatalf("seed user blacklist entry: %v", err)
	}
	if err := repo.Add(context.Background(), "group", "20002", "风险群已封禁"); err != nil {
		t.Fatalf("seed group blacklist entry: %v", err)
	}

	server := httptest.NewServer(application.Handler())
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
	defer response.Body.Close()
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
	repo := permission.NewSQLiteBlacklistRepository(application.Storage().Read, application.Storage().Write)
	if err := repo.Add(context.Background(), "user", "10001", "旧原因"); err != nil {
		t.Fatalf("seed blacklist entry: %v", err)
	}
	seeded, err := repo.Get(context.Background(), "user", "10001")
	if err != nil {
		t.Fatalf("get seeded blacklist entry: %v", err)
	}

	server := httptest.NewServer(application.Handler())
	defer server.Close()

	upsertReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/governance/blacklist/entries", strings.NewReader(`{"entry_type":"user","target_id":"10001","reason":"新原因"}`))
	if err != nil {
		t.Fatalf("create blacklist upsert request: %v", err)
	}
	upsertReq.Header.Set("Authorization", "Bearer "+token)
	upsertReq.Header.Set("Content-Type", "application/json")

	upsertResp, err := server.Client().Do(upsertReq)
	if err != nil {
		t.Fatalf("perform blacklist upsert request: %v", err)
	}
	defer upsertResp.Body.Close()
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

	invalidReq, err := http.NewRequest(http.MethodPost, server.URL+"/api/governance/blacklist/entries", strings.NewReader(`{"entry_type":"user","target_id":"10001","reason":""}`))
	if err != nil {
		t.Fatalf("create invalid blacklist upsert request: %v", err)
	}
	invalidReq.Header.Set("Authorization", "Bearer "+token)
	invalidReq.Header.Set("Content-Type", "application/json")

	invalidResp, err := server.Client().Do(invalidReq)
	if err != nil {
		t.Fatalf("perform invalid blacklist upsert request: %v", err)
	}
	defer invalidResp.Body.Close()
	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected invalid blacklist upsert status: got %d want 400", invalidResp.StatusCode)
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/governance/blacklist/entries/user/10001", nil)
	if err != nil {
		t.Fatalf("create blacklist delete request: %v", err)
	}
	deleteReq.Header.Set("Authorization", "Bearer "+token)

	deleteResp, err := server.Client().Do(deleteReq)
	if err != nil {
		t.Fatalf("perform blacklist delete request: %v", err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected blacklist delete status: got %d want 204", deleteResp.StatusCode)
	}

	missingReq, err := http.NewRequest(http.MethodDelete, server.URL+"/api/governance/blacklist/entries/user/10001", nil)
	if err != nil {
		t.Fatalf("create missing blacklist delete request: %v", err)
	}
	missingReq.Header.Set("Authorization", "Bearer "+token)

	missingResp, err := server.Client().Do(missingReq)
	if err != nil {
		t.Fatalf("perform missing blacklist delete request: %v", err)
	}
	defer missingResp.Body.Close()
	if missingResp.StatusCode != http.StatusNotFound {
		t.Fatalf("unexpected missing blacklist delete status: got %d want 404", missingResp.StatusCode)
	}

	missingBody := decodeBody(t, readAll(t, missingResp))
	errorBody, ok := missingBody["error"].(map[string]any)
	if !ok || errorBody["code"] != "platform.resource_missing" {
		t.Fatalf("unexpected missing blacklist delete body: %#v", missingBody)
	}
}

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
