package management

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/pagination"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil/permissiontest"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/governance"
	"github.com/RayleaBot/RayleaBot/server/internal/bot/permission"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
)

type stubGovernanceEntryRepo struct {
	entries map[string]map[string]permission.Entry
}

func newStubGovernanceEntryRepo() *stubGovernanceEntryRepo {
	return &stubGovernanceEntryRepo{entries: make(map[string]map[string]permission.Entry)}
}

func (s *stubGovernanceEntryRepo) Contains(_ context.Context, scope permission.Scope, entryType, targetID string) (bool, error) {
	_, err := s.Get(context.Background(), scope, entryType, targetID)
	return err == nil, nil
}

func (s *stubGovernanceEntryRepo) Get(_ context.Context, scope permission.Scope, entryType, targetID string) (permission.Entry, error) {
	if items, ok := s.entries[entryType]; ok {
		if entry, ok := items[targetID]; ok {
			return entry, nil
		}
	}
	return permission.Entry{}, permission.ErrGovernanceEntryNotFound
}

func (s *stubGovernanceEntryRepo) Add(_ context.Context, scope permission.Scope, entryType, targetID, reason string) error {
	if s.entries[entryType] == nil {
		s.entries[entryType] = make(map[string]permission.Entry)
	}
	s.entries[entryType][targetID] = permission.Entry{
		Scope:     scope,
		EntryType: entryType,
		TargetID:  targetID,
		Reason:    reason,
		CreatedAt: "2026-04-20T00:00:00Z",
	}
	return nil
}

func (s *stubGovernanceEntryRepo) Remove(_ context.Context, scope permission.Scope, entryType, targetID string) error {
	if _, ok := s.entries[entryType][targetID]; !ok {
		return permission.ErrGovernanceEntryNotFound
	}
	delete(s.entries[entryType], targetID)
	return nil
}

func (s *stubGovernanceEntryRepo) List(_ context.Context, entryType string) ([]permission.Entry, error) {
	items := make([]permission.Entry, 0, len(s.entries[entryType]))
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

func newGovernanceRouter(cfg config.Config, blacklist governance.ManagementEntryRepository, whitelist governance.ManagementEntryRepository, whitelistState permission.WhitelistStateRepository, catalog plugins.CatalogView) *chi.Mux {
	router := chi.NewRouter()
	NewGovernanceHandlersWithService(governance.NewService(governance.Deps{
		CurrentConfig:  func() config.Config { return cfg },
		Plugins:        catalog,
		BlacklistRepo:  blacklist,
		WhitelistRepo:  whitelist,
		WhitelistState: whitelistState,
	})).RegisterProtectedRoutes(router)
	return router
}

func TestGovernanceWhitelistStateRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	router := newGovernanceRouter(config.Config{}, newStubGovernanceEntryRepo(), newStubGovernanceEntryRepo(), &stubWhitelistStateRepo{}, plugincatalog.New(nil))
	req := httptest.NewRequest(http.MethodPut, "/api/governance/whitelist/state", bytes.NewBufferString(`{"enabled":"yes"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", resp.Code, resp.Body.String())
	}
}

func TestGovernanceEntriesRejectLegacyScopes(t *testing.T) {
	t.Parallel()
	router := newGovernanceRouter(config.Config{}, newStubGovernanceEntryRepo(), newStubGovernanceEntryRepo(), &stubWhitelistStateRepo{}, plugincatalog.New(nil))
	for _, list := range []string{"blacklist", "whitelist"} {
		for _, scope := range []struct {
			name, body, query string
		}{
			{"instance", `{"kind":"instance","source_protocol":"onebot11","bot_id":"bot"}`, "kind=instance&source_protocol=onebot11&bot_id=bot"},
			{"adapter", `{"kind":"account","source_protocol":"onebot11","bot_id":"bot","source_adapter":""}`, "kind=account&source_protocol=onebot11&bot_id=bot&source_adapter="},
		} {
			for _, method := range []string{http.MethodPost, http.MethodDelete} {
				t.Run(list+"/"+scope.name+"/"+method, func(t *testing.T) {
					path := "/api/governance/" + list + "/entries"
					body := `{"scope":` + scope.body + `,"entry_type":"user","target_id":"target","reason":"reviewed"}`
					if method == http.MethodDelete {
						path += "/user/target?" + scope.query
						body = ""
					}
					req := httptest.NewRequest(method, path, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					resp := httptest.NewRecorder()
					router.ServeHTTP(resp, req)
					var payload struct {
						Error struct {
							Code string `json:"code"`
						} `json:"error"`
					}
					if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
						t.Fatal(err)
					}
					if resp.Code != http.StatusBadRequest || payload.Error.Code != "platform.invalid_request" {
						t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
					}
				})
			}
		}
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
				{ID: "forecast", Name: "forecast", TriggerType: "exact", Permission: "super_admin", Aliases: []string{"fc"}},
				{ID: "current", Name: "current", TriggerType: "exact"},
			},
		},
	})
	router := newGovernanceRouter(cfg, newStubGovernanceEntryRepo(), newStubGovernanceEntryRepo(), &stubWhitelistStateRepo{}, catalog)

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
	if payload.Commands[0].CommandID != "current" || payload.Commands[0].Command != "current" || payload.Commands[0].Trigger.Type != "exact" || payload.Commands[0].EffectivePermission != "group_admin" || payload.Commands[0].PermissionSource != "default_level" {
		t.Fatalf("unexpected default permission projection: %#v", payload.Commands[0])
	}
	if payload.Commands[1].CommandID != "forecast" || payload.Commands[1].Command != "forecast" || payload.Commands[1].Trigger.Type != "exact" || payload.Commands[1].EffectivePermission != "super_admin" || payload.Commands[1].DeclaredPermission == nil || *payload.Commands[1].DeclaredPermission != "super_admin" {
		t.Fatalf("unexpected declared permission projection: %#v", payload.Commands[1])
	}
}

func (s *stubGovernanceEntryRepo) Page(ctx context.Context, query pagination.Query, entryType string) (permission.EntryPage, error) {
	return permissiontest.Page(ctx, s.List, query, entryType)
}
