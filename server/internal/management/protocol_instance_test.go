package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
	"github.com/RayleaBot/RayleaBot/server/internal/wsevents"
)

type protocolInstanceConfig struct{ config config.Config }

func (s *protocolInstanceConfig) CurrentConfig() config.Config { return s.config }

func TestOneBotManagementQueriesStayWithinExplicitInstance(t *testing.T) {
	t.Parallel()
	source := &protocolInstanceConfig{}
	shells := map[string]*onebot11.Shell{}
	calls := map[string]*atomic.Int32{}
	for _, id := range []string{"first", "second"} {
		count := new(atomic.Int32)
		calls[id] = count
		provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count.Add(1)
			var request onebot11.APICallRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			var data any
			switch request.Action {
			case "get_group_list":
				data = []any{map[string]any{"group_id": "200", "group_name": id + " group"}}
			case "get_friend_list":
				data = []any{map[string]any{"user_id": "300", "nickname": id + " friend"}}
			case "get_group_member_info":
				data = map[string]any{"group_id": "200", "user_id": "300", "nickname": id + " member", "role": "member"}
			default:
				t.Errorf("unexpected action: %s", request.Action)
			}
			if err := json.NewEncoder(w).Encode(map[string]any{"status": "ok", "retcode": 0, "data": data}); err != nil {
				t.Error(err)
			}
		}))
		t.Cleanup(provider.Close)
		settings := config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: provider.URL}}
		source.config.Adapters = append(source.config.Adapters, config.AdapterInstance{ID: id, Type: "onebot11", Enabled: true, OneBot11: &settings})
		shells[id] = onebot11.New(id, settings, config.AdapterConfig{}, nil)
	}
	service := wsevents.NewProtocolService(source, wsevents.ProtocolServiceAdapters{OneBot11: shells})
	router := chi.NewRouter()
	NewProtocolHandlers(service).RegisterProtectedRoutes(router)
	query := func(method, id, suffix, body string, wantStatus int) string {
		t.Helper()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, "/api/adapters/"+id+"/onebot11/"+suffix, strings.NewReader(body))
		router.ServeHTTP(recorder, request)
		if recorder.Code != wantStatus {
			t.Fatalf("%s %s: status=%d body=%s", id, suffix, recorder.Code, recorder.Body.String())
		}
		return recorder.Body.String()
	}
	for _, id := range []string{"second", "first"} {
		beforeFirst, beforeSecond := calls["first"].Load(), calls["second"].Load()
		body := query(http.MethodGet, id, "targets", "", http.StatusOK)
		if !strings.Contains(body, id+" group") || !strings.Contains(body, id+" friend") {
			t.Fatalf("targets from wrong instance: %s", body)
		}
		body = query(http.MethodPost, id, "identities/resolve", `{"items":[{"target_type":"group","target_id":"200","user_id":"300"}]}`, http.StatusOK)
		if !strings.Contains(body, id+" member") {
			t.Fatalf("identity from wrong instance: %s", body)
		}
		wantFirst, wantSecond := beforeFirst, beforeSecond
		if id == "first" {
			wantFirst += 3
		} else {
			wantSecond += 3
		}
		if calls["first"].Load() != wantFirst || calls["second"].Load() != wantSecond {
			t.Fatalf("wrong provider request counts: first=%d second=%d", calls["first"].Load(), calls["second"].Load())
		}
		source.config.Adapters[0], source.config.Adapters[1] = source.config.Adapters[1], source.config.Adapters[0]
	}
	for _, tc := range []struct {
		name string
		id   string
		edit func()
	}{
		{name: "unknown", id: "unknown"},
		{name: "disabled", id: "second", edit: func() { source.config.Adapters[1].Enabled = false }},
		{name: "mismatched protocol", id: "first", edit: func() { source.config.Adapters[0].Type = "qqofficial" }},
		{name: "removed", id: "second", edit: func() { source.config.Adapters = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.edit != nil {
				tc.edit()
			}
			beforeFirst, beforeSecond := calls["first"].Load(), calls["second"].Load()
			for _, operation := range []struct{ method, suffix, body string }{
				{http.MethodGet, "targets", ""},
				{http.MethodPost, "identities/resolve", `{"items":[{"target_type":"group","target_id":"200","user_id":"300"}]}`},
			} {
				body := query(operation.method, tc.id, operation.suffix, operation.body, http.StatusBadRequest)
				if !strings.Contains(body, `"code":"platform.invalid_request"`) {
					t.Fatalf("missing stable selector error: %s", body)
				}
			}
			if calls["first"].Load() != beforeFirst || calls["second"].Load() != beforeSecond {
				t.Fatal("rejected selection reached a provider")
			}
		})
	}
}
