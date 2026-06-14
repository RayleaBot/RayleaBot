package governanceapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type stubBlacklistRepo struct {
	entries map[string]map[string]permission.BlacklistEntry
}

func newStubBlacklistRepo() *stubBlacklistRepo {
	return &stubBlacklistRepo{entries: make(map[string]map[string]permission.BlacklistEntry)}
}

func (s *stubBlacklistRepo) IsBlacklisted(_ context.Context, entryType, targetID string) (bool, error) {
	_, err := s.Get(context.Background(), entryType, targetID)
	return err == nil, nil
}

func (s *stubBlacklistRepo) Get(_ context.Context, entryType, targetID string) (permission.BlacklistEntry, error) {
	if items, ok := s.entries[entryType]; ok {
		if entry, ok := items[targetID]; ok {
			return entry, nil
		}
	}
	return permission.BlacklistEntry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubBlacklistRepo) Add(_ context.Context, entryType, targetID, reason string) error {
	if s.entries[entryType] == nil {
		s.entries[entryType] = make(map[string]permission.BlacklistEntry)
	}
	s.entries[entryType][targetID] = permission.BlacklistEntry{
		EntryType: entryType,
		TargetID:  targetID,
		Reason:    reason,
		CreatedAt: "2026-04-20T00:00:00Z",
	}
	return nil
}

func (s *stubBlacklistRepo) Remove(_ context.Context, entryType, targetID string) error {
	if _, ok := s.entries[entryType][targetID]; !ok {
		return permission.ErrGovernanceEntryNotFound
	}
	delete(s.entries[entryType], targetID)
	return nil
}

func (s *stubBlacklistRepo) List(_ context.Context, entryType string) ([]permission.BlacklistEntry, error) {
	items := make([]permission.BlacklistEntry, 0, len(s.entries[entryType]))
	for _, entry := range s.entries[entryType] {
		items = append(items, entry)
	}
	return items, nil
}

type stubWhitelistRepo struct {
	entries map[string]map[string]permission.WhitelistEntry
}

func newStubWhitelistRepo() *stubWhitelistRepo {
	return &stubWhitelistRepo{entries: make(map[string]map[string]permission.WhitelistEntry)}
}

func (s *stubWhitelistRepo) IsWhitelisted(_ context.Context, entryType, targetID string) (bool, error) {
	_, err := s.Get(context.Background(), entryType, targetID)
	return err == nil, nil
}

func (s *stubWhitelistRepo) Get(_ context.Context, entryType, targetID string) (permission.WhitelistEntry, error) {
	if items, ok := s.entries[entryType]; ok {
		if entry, ok := items[targetID]; ok {
			return entry, nil
		}
	}
	return permission.WhitelistEntry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubWhitelistRepo) Add(_ context.Context, entryType, targetID, reason string) error {
	if s.entries[entryType] == nil {
		s.entries[entryType] = make(map[string]permission.WhitelistEntry)
	}
	s.entries[entryType][targetID] = permission.WhitelistEntry{
		EntryType: entryType,
		TargetID:  targetID,
		Reason:    reason,
		CreatedAt: "2026-04-20T00:00:00Z",
	}
	return nil
}

func (s *stubWhitelistRepo) Remove(_ context.Context, entryType, targetID string) error {
	if _, ok := s.entries[entryType][targetID]; !ok {
		return permission.ErrGovernanceEntryNotFound
	}
	delete(s.entries[entryType], targetID)
	return nil
}

func (s *stubWhitelistRepo) List(_ context.Context, entryType string) ([]permission.WhitelistEntry, error) {
	items := make([]permission.WhitelistEntry, 0, len(s.entries[entryType]))
	for _, entry := range s.entries[entryType] {
		items = append(items, entry)
	}
	return items, nil
}

type stubWhitelistStateRepo struct {
	enabled bool
}

func (s *stubWhitelistStateRepo) Enabled(context.Context) (bool, error) {
	return s.enabled, nil
}

func (s *stubWhitelistStateRepo) SetEnabled(_ context.Context, enabled bool) error {
	s.enabled = enabled
	return nil
}

func newGovernanceRouter(cfg config.Config, blacklist permission.BlacklistRepository, whitelist permission.WhitelistRepository, whitelistState permission.WhitelistStateRepository, catalog plugins.CatalogView) *chi.Mux {
	router := chi.NewRouter()
	NewHandlers(governance.Deps{
		CurrentConfig:  func() config.Config { return cfg },
		Plugins:        catalog,
		BlacklistRepo:  blacklist,
		WhitelistRepo:  whitelist,
		WhitelistState: whitelistState,
	}).RegisterProtectedRoutes(router)
	return router
}

func TestGovernanceBlacklistAndWhitelistRoundTrip(t *testing.T) {
	t.Parallel()

	blacklist := newStubBlacklistRepo()
	whitelist := newStubWhitelistRepo()
	whitelistState := &stubWhitelistStateRepo{}
	router := newGovernanceRouter(config.Config{}, blacklist, whitelist, whitelistState, plugincatalog.New(nil))

	upsertBody := bytes.NewBufferString(`{"entry_type":"user","target_id":"1001","reason":"spam"}`)
	upsertReq := httptest.NewRequest(http.MethodPost, "/api/governance/blacklist/entries", upsertBody)
	upsertReq.Header.Set("Content-Type", "application/json")
	upsertResp := httptest.NewRecorder()
	router.ServeHTTP(upsertResp, upsertReq)
	if upsertResp.Code != http.StatusOK {
		t.Fatalf("blacklist upsert status = %d, want 200; body=%s", upsertResp.Code, upsertResp.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/governance/blacklist", nil)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("blacklist list status = %d, want 200; body=%s", listResp.Code, listResp.Body.String())
	}

	var blacklistPayload governance.BlacklistSnapshot
	if err := json.Unmarshal(listResp.Body.Bytes(), &blacklistPayload); err != nil {
		t.Fatalf("decode blacklist response: %v", err)
	}
	if len(blacklistPayload.UserEntries) != 1 || blacklistPayload.UserEntries[0].TargetID != "1001" {
		t.Fatalf("unexpected blacklist payload: %#v", blacklistPayload)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/governance/blacklist/entries/user/1001", nil)
	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("blacklist delete status = %d, want 204; body=%s", deleteResp.Code, deleteResp.Body.String())
	}

	stateBody := bytes.NewBufferString(`{"enabled":true}`)
	stateReq := httptest.NewRequest(http.MethodPut, "/api/governance/whitelist/state", stateBody)
	stateReq.Header.Set("Content-Type", "application/json")
	stateResp := httptest.NewRecorder()
	router.ServeHTTP(stateResp, stateReq)
	if stateResp.Code != http.StatusOK {
		t.Fatalf("whitelist state status = %d, want 200; body=%s", stateResp.Code, stateResp.Body.String())
	}
	if !whitelistState.enabled {
		t.Fatal("expected whitelist state to be enabled")
	}

	whitelistBody := bytes.NewBufferString(`{"entry_type":"group","target_id":"2001","reason":"approved"}`)
	whitelistReq := httptest.NewRequest(http.MethodPost, "/api/governance/whitelist/entries", whitelistBody)
	whitelistReq.Header.Set("Content-Type", "application/json")
	whitelistResp := httptest.NewRecorder()
	router.ServeHTTP(whitelistResp, whitelistReq)
	if whitelistResp.Code != http.StatusOK {
		t.Fatalf("whitelist upsert status = %d, want 200; body=%s", whitelistResp.Code, whitelistResp.Body.String())
	}

	getWhitelistReq := httptest.NewRequest(http.MethodGet, "/api/governance/whitelist", nil)
	getWhitelistResp := httptest.NewRecorder()
	router.ServeHTTP(getWhitelistResp, getWhitelistReq)
	if getWhitelistResp.Code != http.StatusOK {
		t.Fatalf("whitelist list status = %d, want 200; body=%s", getWhitelistResp.Code, getWhitelistResp.Body.String())
	}

	var whitelistPayload governance.WhitelistSnapshot
	if err := json.Unmarshal(getWhitelistResp.Body.Bytes(), &whitelistPayload); err != nil {
		t.Fatalf("decode whitelist response: %v", err)
	}
	if !whitelistPayload.Enabled || len(whitelistPayload.GroupEntries) != 1 || whitelistPayload.GroupEntries[0].TargetID != "2001" {
		t.Fatalf("unexpected whitelist payload: %#v", whitelistPayload)
	}
}

func TestGovernanceWhitelistStateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	router := newGovernanceRouter(config.Config{}, newStubBlacklistRepo(), newStubWhitelistRepo(), &stubWhitelistStateRepo{}, plugincatalog.New(nil))
	req := httptest.NewRequest(http.MethodPut, "/api/governance/whitelist/state", bytes.NewBufferString(`{"enabled":"yes"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", resp.Code, resp.Body.String())
	}
}

func TestGovernanceCommandPolicyProjection(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Permission: config.PermissionConfig{
			DefaultLevel: "group_admin",
		},
		User: config.UserConfig{
			CommandRateLimit: "5/60s",
			CooldownReply:    false,
		},
		Group: config.GroupConfig{
			CommandRateLimit: "9/60s",
		},
	}
	catalog := plugincatalog.New([]plugins.Snapshot{
		{
			PluginID:          "weather",
			Name:              "Weather",
			Valid:             true,
			RegistrationState: "installed",
			DesiredState:      "enabled",
			Commands: []plugins.Command{
				{Name: "forecast", Permission: "super_admin", Aliases: []string{"fc"}},
				{Name: "current"},
			},
		},
	})
	router := newGovernanceRouter(cfg, newStubBlacklistRepo(), newStubWhitelistRepo(), &stubWhitelistStateRepo{}, catalog)

	req := httptest.NewRequest(http.MethodGet, "/api/governance/command-policy", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.Code, resp.Body.String())
	}

	var payload governance.CommandPolicyResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.DefaultLevel != "group_admin" {
		t.Fatalf("default_level = %q, want group_admin", payload.DefaultLevel)
	}
	if payload.Cooldown.UserCommandRateLimit != "5/60s" || payload.Cooldown.GroupCommandRateLimit != "9/60s" {
		t.Fatalf("unexpected cooldown snapshot: %#v", payload.Cooldown)
	}
	if len(payload.Commands) != 2 {
		t.Fatalf("len(commands) = %d, want 2", len(payload.Commands))
	}
	if payload.Commands[0].Command != "current" || payload.Commands[0].CommandSource != "manifest" || payload.Commands[0].EffectivePermission != "group_admin" || payload.Commands[0].PermissionSource != "default_level" {
		t.Fatalf("unexpected default permission projection: %#v", payload.Commands[0])
	}
	if payload.Commands[1].Command != "forecast" || payload.Commands[1].CommandSource != "manifest" || payload.Commands[1].EffectivePermission != "super_admin" || payload.Commands[1].DeclaredPermission == nil || *payload.Commands[1].DeclaredPermission != "super_admin" {
		t.Fatalf("unexpected declared permission projection: %#v", payload.Commands[1])
	}
}
