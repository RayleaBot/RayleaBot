package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/bot/adapters"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/go-chi/chi/v5"
)

func managementAccountDirectory(t *testing.T) (*adapters.BotAccountDirectory, func([]adapters.BotAccountConnection)) {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "accounts.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	var connections []adapters.BotAccountConnection
	directory, err := adapters.NewBotAccountDirectory(t.Context(), adapters.BotAccountDirectoryDeps{Store: store, Connections: func() []adapters.BotAccountConnection { return connections }})
	if err != nil {
		t.Fatal(err)
	}
	return directory, func(next []adapters.BotAccountConnection) {
		connections = next
		if err := directory.Observe(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBotAccountHTTP(t *testing.T) {
	directory, update := managementAccountDirectory(t)
	router := chi.NewRouter()
	NewBotAccountHandlers(directory).RegisterProtectedRoutes(router)
	request := func(method, url string, status int) *httptest.ResponseRecorder {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(method, url, nil))
		if response.Code != status {
			t.Fatalf("%s %s = %d %s", method, url, response.Code, response.Body.String())
		}
		return response
	}
	empty := request("GET", "/api/bot-accounts", 200)
	var page adapters.BotAccountPage
	if err := json.Unmarshal(empty.Body.Bytes(), &page); err != nil || page.Items == nil || len(page.Items) != 0 || page.Total != 0 || page.NextCursor != "" {
		t.Fatalf("empty=%s %v", empty.Body.String(), err)
	}
	request("GET", "/api/bot-accounts/onebot11/missing", 404)
	for _, query := range []string{"state=bad", "source_protocol=bad", "limit=101", "cursor=01"} {
		request("GET", "/api/bot-accounts?"+query, 400)
	}
	connections := []adapters.BotAccountConnection{
		{AdapterDescriptor: adapters.AdapterDescriptor{ID: "x", DisplayName: "Alpha", Protocol: "onebot11", Enabled: true, State: "connected", Identity: &adapters.AdapterIdentity{ID: "2", Name: "Two"}}},
		{AdapterDescriptor: adapters.AdapterDescriptor{ID: "y", DisplayName: "Beta", Protocol: "onebot11", Enabled: true, State: "connected", Identity: &adapters.AdapterIdentity{ID: "10", Name: "Ten"}}},
	}
	update(connections)
	first := request("GET", "/api/bot-accounts?limit=1", 200)
	if err := json.Unmarshal(first.Body.Bytes(), &page); err != nil || page.Total != 2 || page.NextCursor != "1" || page.Items[0].BotID != "10" {
		t.Fatalf("page=%s %v", first.Body.String(), err)
	}
	second := request("GET", "/api/bot-accounts?limit=1&cursor=1", 200)
	page = adapters.BotAccountPage{}
	if err := json.Unmarshal(second.Body.Bytes(), &page); err != nil || page.Items[0].BotID != "2" || page.NextCursor != "" {
		t.Fatalf("page=%s %v", second.Body.String(), err)
	}
	filtered := request("GET", "/api/bot-accounts?query=ALPHA&source_protocol=onebot11&state=online", 200)
	if err := json.Unmarshal(filtered.Body.Bytes(), &page); err != nil || page.Total != 1 || page.Items[0].BotID != "2" {
		t.Fatalf("filtered=%s %v", filtered.Body.String(), err)
	}
	request("GET", "/api/bot-accounts/onebot11/2", 200)
	conflict := request("DELETE", "/api/bot-accounts/onebot11/2", 409)
	if !strings.Contains(conflict.Body.String(), "platform.state_conflict") {
		t.Fatal(conflict.Body.String())
	}
	update(nil)
	removed := request("DELETE", "/api/bot-accounts/onebot11/2", 204)
	if removed.Body.Len() != 0 {
		t.Fatal("204 contained a body")
	}
	request("GET", "/api/bot-accounts/onebot11/2", 404)
	update(connections)
	request("GET", "/api/bot-accounts/onebot11/2", 200)
	for _, id := range []string{"opaque/id", "literal%2Fvalue", "id:with colon"} {
		update([]adapters.BotAccountConnection{{AdapterDescriptor: adapters.AdapterDescriptor{ID: "qq", Protocol: "qqofficial", DisplayName: "QQ", Enabled: true, State: "connected", Identity: &adapters.AdapterIdentity{ID: id}}}})
		request("GET", "/api/bot-accounts/qqofficial/"+url.PathEscape(id), 200)
	}
}

type accountActionInvoker struct {
	calls   int
	page    plugins.ManagementActionContext
	payload map[string]any
}

func (i *accountActionInvoker) InvokeManagementAction(_ context.Context, _, _ string, payload map[string]any, page plugins.ManagementActionContext) (map[string]any, error) {
	i.calls++
	i.page, i.payload = page, payload
	return map[string]any{}, nil
}

func TestManagementActionPageValidation(t *testing.T) {
	directory, update := managementAccountDirectory(t)
	update([]adapters.BotAccountConnection{{AdapterDescriptor: adapters.AdapterDescriptor{ID: "x", Protocol: "onebot11", DisplayName: "X", Enabled: true, State: "connected", Identity: &adapters.AdapterIdentity{ID: "account"}}}})
	update(nil)
	catalog := catalog.New([]plugins.Snapshot{{PluginID: "test", Valid: true, RegistrationState: "installed", ManagementUI: &plugins.ManagementUI{Entry: "ui/index.html", Pages: []plugins.ManagementUIPage{{ID: "global"}, {ID: "records", Scope: "bot_account"}}}}})
	invoker := &accountActionInvoker{}
	handler := NewPluginManagementUIHandlers(PluginManagementUIDeps{Plugins: catalog, ActionInvoker: invoker, BotAccounts: directory})
	router := chi.NewRouter()
	handler.RegisterProtectedRoutes(router)
	for _, test := range []struct {
		name, fields string
		status       int
	}{
		{"unknown page", `"page_id":"missing"`, 400},
		{"missing bot", `"page_id":"records"`, 400},
		{"global with bot", `"page_id":"global","bot":{"source_protocol":"onebot11","bot_id":"account"}`, 400},
		{"bot without page", `"bot":{"source_protocol":"onebot11","bot_id":"account"}`, 400},
		{"unknown account", `"page_id":"records","bot":{"source_protocol":"onebot11","bot_id":"missing"}`, 400},
		{"unknown protocol", `"page_id":"records","bot":{"source_protocol":"other","bot_id":"account"}`, 400},
		{"incomplete bot", `"page_id":"records","bot":{"source_protocol":"onebot11"}`, 400},
		{"extra bot field", `"page_id":"records","bot":{"source_protocol":"onebot11","bot_id":"account","extra":1}`, 400},
		{"null bot", `"page_id":"records","bot":null`, 400},
		{"null page", `"page_id":null`, 400},
		{"global", `"page_id":"global"`, 200},
		{"legacy nested context", `"payload":{"bot":{"bot_id":"spoof"},"page_id":"spoof"}`, 200},
		{"account with nested context", `"page_id":"records","bot":{"source_protocol":"onebot11","bot_id":"account"},"payload":{"bot":{"bot_id":"spoof"},"page_id":"spoof"}`, 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := invoker.calls
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/plugins/test/management/actions", strings.NewReader(`{"action":"save",`+test.fields+`}`)))
			if response.Code != test.status {
				t.Fatalf("response=%d %s", response.Code, response.Body.String())
			}
			if test.status == 400 {
				if invoker.calls != before || !strings.Contains(response.Body.String(), "platform.invalid_request") {
					t.Fatal("invalid action was delivered or mapped incorrectly")
				}
			}
			if test.name == "account with nested context" {
				if invoker.page.PageID != "records" || invoker.page.Bot == nil || invoker.page.Bot.BotID != "account" || invoker.payload["page_id"] != "spoof" {
					t.Fatalf("context=%+v payload=%v", invoker.page, invoker.payload)
				}
			} else if test.name == "legacy nested context" && (invoker.page.PageID != "" || invoker.page.Bot != nil) {
				t.Fatal("nested context was promoted")
			}
		})
	}
}
