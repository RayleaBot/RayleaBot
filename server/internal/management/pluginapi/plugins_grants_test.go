package pluginapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

func TestGrantHandler_ValidCapability(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]plugins.Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"event.subscribe", "http.request"},
		RequiredPermissions:  []string{"http.request"},
		OptionalPermissions:  []string{"event.subscribe"},
	}}, repo)

	body, _ := json.Marshal(grantRequest{Capability: "http.request"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp grantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Capability != "http.request" {
		t.Fatalf("capability = %q, want http.request", resp.Capability)
	}
	if resp.Source != string(plugins.GrantSourcePersisted) {
		t.Fatalf("source = %q, want %q", resp.Source, plugins.GrantSourcePersisted)
	}
	if resp.GrantedAt == nil || *resp.GrantedAt == "" {
		t.Fatalf("granted_at = %#v, want populated timestamp", resp.GrantedAt)
	}
	if resp.ExpiresAt != nil {
		t.Fatalf("expires_at = %v, want nil for permanent grant", *resp.ExpiresAt)
	}
}

func TestGrantHandler_AcceptsMultiSegmentCapability(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]plugins.Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"message.history.get"},
		RequiredPermissions:  []string{"message.history.get"},
	}}, repo)

	body, _ := json.Marshal(grantRequest{Capability: "message.history.get"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}

func TestGrantHandler_AcceptsProviderCapability(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]plugins.Snapshot{{
		PluginID:            "weather",
		Valid:               true,
		RegistrationState:   "installed",
		DesiredState:        "disabled",
		RuntimeState:        "stopped",
		OptionalPermissions: []string{"provider.napcat.group.sign.set"},
	}}, repo)

	body, _ := json.Marshal(grantRequest{Capability: "provider.napcat.group.sign.set"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}

func TestGrantHandler_RejectsInvalidCapabilityFormat(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]plugins.Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"event.subscribe"},
	}}, repo)

	for _, badCap := range []string{"INVALID", "no-dot", "has.Upper", "has.num3rs", "123.abc"} {
		body, _ := json.Marshal(grantRequest{Capability: badCap})
		req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("capability %q: status = %d, want 400", badCap, rec.Code)
		}
	}
}

func TestGrantHandler_RejectsUndeclaredCapability(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]plugins.Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"event.subscribe"},
		RequiredPermissions:  []string{"event.subscribe"},
	}}, repo)

	// http.request is valid format but not declared in this plugin's manifest.
	body, _ := json.Marshal(grantRequest{Capability: "http.request"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != codeInvalidRequest {
		t.Fatalf("error.code = %q, want %q", env.Error.Code, codeInvalidRequest)
	}
}

func TestGrantHandler_AcceptsOptionalPermission(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]plugins.Snapshot{{
		PluginID:            "weather",
		Valid:               true,
		RegistrationState:   "installed",
		DesiredState:        "disabled",
		RuntimeState:        "stopped",
		OptionalPermissions: []string{"logger.write"},
	}}, repo)

	body, _ := json.Marshal(grantRequest{Capability: "logger.write"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
}

func TestGrantHandler_PluginNotFound(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter(nil, repo)

	body, _ := json.Marshal(grantRequest{Capability: "http.request"})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/nonexistent/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGrantHandler_AcceptsFutureExpiry(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]plugins.Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"logger.write"},
		OptionalPermissions:  []string{"logger.write"},
	}}, repo)

	expiresAt := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	body, _ := json.Marshal(grantRequest{Capability: "logger.write", ExpiresAt: &expiresAt})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp grantResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ExpiresAt == nil || *resp.ExpiresAt != expiresAt {
		t.Fatalf("expires_at = %#v, want %q", resp.ExpiresAt, expiresAt)
	}
	if resp.Source != string(plugins.GrantSourcePersisted) {
		t.Fatalf("source = %q, want %q", resp.Source, plugins.GrantSourcePersisted)
	}
}

func TestGrantHandler_RejectsInvalidExpiry(t *testing.T) {
	t.Parallel()

	repo := &stubGrantRepository{}
	router := grantsRouter([]plugins.Snapshot{{
		PluginID:             "weather",
		Valid:                true,
		RegistrationState:    "installed",
		DesiredState:         "disabled",
		RuntimeState:         "stopped",
		DeclaredCapabilities: []string{"http.request"},
		RequiredPermissions:  []string{"http.request"},
	}}, repo)

	expiresAt := "2026-03-22T10:00:00+08:00"
	body, _ := json.Marshal(grantRequest{Capability: "http.request", ExpiresAt: &expiresAt})
	req := httptest.NewRequest(http.MethodPost, "/api/plugins/weather/grants", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}

	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Code != codeInvalidRequest {
		t.Fatalf("error.code = %q, want %q", env.Error.Code, codeInvalidRequest)
	}
}

func TestListGrantsHandler_OmitsExpiredGrant(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	repo := &stubGrantRepository{
		grants: map[string][]plugins.PluginGrant{
			"weather": {
				{
					PluginID:   "weather",
					Capability: "http.request",
					GrantedAt:  now,
				},
				{
					PluginID:   "weather",
					Capability: "logger.write",
					GrantedAt:  now,
					ExpiresAt:  &future,
				},
				{
					PluginID:   "weather",
					Capability: "storage.file",
					GrantedAt:  now,
					ExpiresAt:  &past,
				},
			},
		},
	}
	router := grantsRouter([]plugins.Snapshot{{
		PluginID:          "weather",
		Valid:             true,
		RegistrationState: "installed",
		DesiredState:      "disabled",
		RuntimeState:      "stopped",
	}}, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/weather/grants", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp grantsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(resp.Items))
	}
	if resp.Items[0].Source != string(plugins.GrantSourcePersisted) || resp.Items[0].GrantedAt == nil {
		t.Fatalf("unexpected first grant: %#v", resp.Items[0])
	}
	if resp.Items[1].ExpiresAt == nil {
		t.Fatalf("expires_at = nil, want populated expiry")
	}
}

func TestListGrantsHandler_ReturnsEffectiveGrantSources(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubGrantRepository{
		grants: map[string][]plugins.PluginGrant{
			"weather": {{
				PluginID:   "weather",
				Capability: "logger.write",
				GrantedAt:  now,
			}},
		},
	}
	router := grantsRouterWithAutoGrants([]plugins.Snapshot{{
		PluginID:            "weather",
		Valid:               true,
		RegistrationState:   "installed",
		DesiredState:        "enabled",
		RuntimeState:        "running",
		RequiredPermissions: []string{"scheduler.create"},
		OptionalPermissions: []string{"logger.write"},
	}}, repo, []string{"scheduler.create"})

	req := httptest.NewRequest(http.MethodGet, "/api/plugins/weather/grants", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}

	var resp grantsListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(resp.Items))
	}
	if resp.Items[0].Capability != "logger.write" || resp.Items[0].Source != string(plugins.GrantSourcePersisted) {
		t.Fatalf("unexpected persisted grant: %#v", resp.Items[0])
	}
	if resp.Items[1].Capability != "scheduler.create" || resp.Items[1].Source != string(plugins.GrantSourceConfigAuto) {
		t.Fatalf("unexpected config auto grant: %#v", resp.Items[1])
	}
	if resp.Items[1].GrantedAt != nil {
		t.Fatalf("config auto granted_at = %#v, want nil", resp.Items[1].GrantedAt)
	}
}

func TestListGrantsHandler_ReturnsBuiltinAutoGrant(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name                string
		pluginID            string
		requiredPermissions []string
		wantCapabilities    []string
	}{
		{
			name:                "echo",
			pluginID:            "raylea.echo",
			requiredPermissions: []string{"message.send"},
			wantCapabilities:    []string{"message.send"},
		},
		{
			name:                "fortune",
			pluginID:            "raylea.fortune",
			requiredPermissions: []string{"message.send", "render.image", "storage.kv"},
			wantCapabilities:    []string{"message.send", "render.image", "storage.kv"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			router := grantsRouter([]plugins.Snapshot{{
				PluginID:            tc.pluginID,
				Valid:               true,
				SourceRoot:          "plugins/builtin",
				RegistrationState:   "installed",
				DesiredState:        "enabled",
				RuntimeState:        "running",
				RequiredPermissions: tc.requiredPermissions,
			}}, nil)

			req := httptest.NewRequest(http.MethodGet, "/api/plugins/"+tc.pluginID+"/grants", nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}

			var resp grantsListResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}

			gotCapabilities := make([]string, 0, len(resp.Items))
			for _, item := range resp.Items {
				if item.Source != string(plugins.GrantSourceBuiltinAuto) {
					t.Fatalf("unexpected builtin auto grant source: %#v", item)
				}
				if item.GrantedAt != nil || item.ExpiresAt != nil {
					t.Fatalf("builtin auto timestamps should be nil: %#v", item)
				}
				gotCapabilities = append(gotCapabilities, item.Capability)
			}
			if !reflect.DeepEqual(gotCapabilities, tc.wantCapabilities) {
				t.Fatalf("builtin auto capabilities = %#v, want %#v", gotCapabilities, tc.wantCapabilities)
			}
		})
	}
}
