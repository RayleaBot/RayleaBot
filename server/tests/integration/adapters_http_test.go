package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	internalapp "github.com/RayleaBot/RayleaBot/server/internal/app"
	internalconfig "github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

// Two OneBot instances are the case the adapters change exists for: each runs
// on its own, listens on its own ingress path and holds its own token.
func newTestAppWithTwoOneBotAdapters(t *testing.T) (*internalapp.App, *httptest.Server) {
	t.Helper()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		primary := testutil.ConfigDocumentAdapterInstance(t, input, internalconfig.DefaultOneBot11AdapterID)
		primary["enabled"] = true
		primaryReverse := testutil.ConfigDocumentOneBot(t, input)["reverse_ws"].(map[string]any)
		primaryReverse["enabled"] = true
		primaryReverse["url"] = "ws://127.0.0.1:8080/onebot/primary"
		primaryReverse["access_token"] = "primary-token"

		input["adapters"] = append(input["adapters"].([]any), testutil.OneBotAdapterDocument("second-bot", true, map[string]any{
			"reverse_ws": map[string]any{
				"enabled":                   true,
				"url":                       "ws://127.0.0.1:8080/onebot/second",
				"access_token":              "second-token",
				"access_token_query_compat": false,
			},
			"forward_ws": map[string]any{"enabled": false, "url": "", "access_token": "", "access_token_query_compat": false},
			"http_api":   map[string]any{"enabled": false, "url": "", "access_token": ""},
			"webhook":    map[string]any{"enabled": false, "url": "", "access_token": "", "access_token_query_compat": false},
		}))
	}, deterministicAuthOptions()...)

	return application, newManagementTestServer(t, application.Handler())
}

func TestAdaptersListsEveryConfiguredInstance(t *testing.T) {
	t.Parallel()

	application, server := newTestAppWithTwoOneBotAdapters(t)
	defer server.Close()
	token := issueLoginToken(t, application)

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/adapters", nil)
	if err != nil {
		t.Fatalf("create adapters request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("perform adapters request: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("adapters status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var body struct {
		Adapters []struct {
			ID          string `json:"id"`
			Protocol    string `json:"protocol"`
			DisplayName string `json:"display_name"`
			Enabled     bool   `json:"enabled"`
		} `json:"adapters"`
		AvailableProtocols []struct {
			Protocol string `json:"protocol"`
		} `json:"available_protocols"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode adapters response: %v", err)
	}

	if len(body.Adapters) != 2 {
		t.Fatalf("listed %d adapters, want both configured instances: %#v", len(body.Adapters), body.Adapters)
	}
	if body.Adapters[0].ID != internalconfig.DefaultOneBot11AdapterID || body.Adapters[1].ID != "second-bot" {
		t.Fatalf("adapter ids = %q, %q, want them in configuration order", body.Adapters[0].ID, body.Adapters[1].ID)
	}
	// Two instances of one protocol have to be distinguishable on the page.
	if body.Adapters[0].DisplayName == body.Adapters[1].DisplayName {
		t.Fatalf("both instances rendered as %q", body.Adapters[0].DisplayName)
	}
	// A protocol stays addable once an instance of it exists.
	if len(body.AvailableProtocols) == 0 {
		t.Fatal("no protocol was offered as addable")
	}
}

func TestAdapterIngressIsAddressedAndAuthenticatedPerInstance(t *testing.T) {
	t.Parallel()

	_, server := newTestAppWithTwoOneBotAdapters(t)
	defer server.Close()

	cases := []struct {
		name       string
		path       string
		token      string
		wantStatus int
	}{
		{
			name:       "primary rejects the other instance's token",
			path:       "/api/adapters/onebot11/reverse-ws",
			token:      "second-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "second instance rejects the primary's token",
			path:       "/api/adapters/second-bot/reverse-ws",
			token:      "primary-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			// An id that names no running adapter has no ingress at all, which
			// is a different answer from failing authentication against a real
			// one.
			name:       "unknown instance has no ingress",
			path:       "/api/adapters/no-such-bot/reverse-ws",
			token:      "primary-token",
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodGet, server.URL+testCase.path, nil)
			if err != nil {
				t.Fatalf("create ingress request: %v", err)
			}
			request.Header.Set("Authorization", "Bearer "+testCase.token)
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatalf("perform ingress request: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != testCase.wantStatus {
				t.Fatalf("ingress status = %d, want %d", response.StatusCode, testCase.wantStatus)
			}
		})
	}
}

func TestAdapterWebhookIngressAuthenticatesAgainstItsOwnInstance(t *testing.T) {
	t.Parallel()

	application, _, _ := newTestAppWithConfigMutation(t, func(input map[string]any) {
		primary := testutil.ConfigDocumentAdapterInstance(t, input, internalconfig.DefaultOneBot11AdapterID)
		primary["enabled"] = true
		webhook := testutil.ConfigDocumentOneBot(t, input)["webhook"].(map[string]any)
		webhook["enabled"] = true
		webhook["url"] = "https://bot.example.com/webhook"
		webhook["access_token"] = "primary-webhook-token"
	}, deterministicAuthOptions()...)
	server := newManagementTestServer(t, application.Handler())
	defer server.Close()

	payload, err := json.Marshal(map[string]any{"post_type": "meta_event", "meta_event_type": "heartbeat"})
	if err != nil {
		t.Fatalf("encode webhook body: %v", err)
	}

	for _, testCase := range []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "wrong token", token: "wrong-token", wantStatus: http.StatusUnauthorized},
		{name: "own token", token: "primary-webhook-token", wantStatus: http.StatusAccepted},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, server.URL+"/api/adapters/onebot11/webhook", bytes.NewReader(payload))
			if err != nil {
				t.Fatalf("create webhook request: %v", err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+testCase.token)
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatalf("perform webhook request: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != testCase.wantStatus {
				t.Fatalf("webhook status = %d, want %d", response.StatusCode, testCase.wantStatus)
			}
		})
	}
}
