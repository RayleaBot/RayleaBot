package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/app"
	"github.com/RayleaBot/RayleaBot/server/internal/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/configruntime"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginstore"
)

func TestAppCompositionSharesSettingsAcrossHTTPRuntimeAndIngress(t *testing.T) {
	t.Parallel()
	application, _, _ := newTestAppWithOptions(t, func(input map[string]any) {
		input["database"].(map[string]any)["path"] = filepath.Join(t.TempDir(), "state.db")
		input["adapters"] = []any{}
	}, func(options *app.Options, _ string) {
		manifestPath := filepath.Join(options.PluginRoots[0].Path, "raylea.echo", "info.json")
		payload, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		var manifest map[string]any
		if err := json.Unmarshal(payload, &manifest); err != nil {
			t.Fatal(err)
		}
		manifest["default_config"] = map[string]any{"fixture_composition": true, "value": "default", "commands": []any{"echo"}}
		manifest["events"] = []any{"message.group", "message.private", "config.changed"}
		command := manifest["commands"].([]any)[0].(map[string]any)
		command["trigger"] = map[string]any{"type": "setting", "settings_key": "commands"}
		payload, err = json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestPath, payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}, deterministicAuthOptions()...)
	token := issueLoginToken(t, application)
	server := newManagementTestServer(t, application.Handler())
	t.Cleanup(server.Close)
	request := func(method, path string, document any) map[string]any {
		t.Helper()
		var payload []byte
		var err error
		if document != nil {
			payload, err = json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = response.Body.Close() }()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s %s: status %d: %s", method, path, response.StatusCode, body)
		}
		return decodeBody(t, body)
	}

	states, cancelStates := application.Plugins().Subscribe(16)
	defer cancelStates()
	logs, cancelLogs := application.Logs().Subscribe(64)
	defer cancelLogs()
	request(http.MethodPost, "/api/plugins/raylea.echo/enable", nil)
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for running := false; !running; {
		select {
		case state := <-states:
			if state.RuntimeErrorCode != "" {
				t.Fatalf("native fixture failed: %#v", state)
			}
			running = state.PluginID == "raylea.echo" && state.RuntimeState == "running"
		case <-deadline.C:
			t.Fatal("native fixture did not initialize")
		}
	}
	repository, err := pluginstore.NewConfigSQLiteRepository(application.Storage())
	if err != nil {
		t.Fatal(err)
	}
	stored, err := repository.ReadAll(context.Background(), "raylea.echo")
	if err != nil || len(stored) != 0 {
		t.Fatalf("initialization persisted default overrides: %#v %v", stored, err)
	}

	settings := request(http.MethodPut, "/api/plugins/raylea.echo/settings", map[string]any{"values": map[string]any{"value": "http", "commands": []any{"say"}}})
	if values := settings["values"].(map[string]any); values["value"] != "http" {
		t.Fatalf("settings response: %#v", values)
	}
	waitCompositionLog(t, logs, func(entry logging.Summary) bool {
		return entry.Details["fixture_event"] == "config.changed" && entry.Details["fixture_value"] == "http"
	})
	snapshot, _ := application.Plugins().Get("raylea.echo")
	if len(snapshot.Commands) != 1 || snapshot.Commands[0].Name != "say" {
		t.Fatalf("HTTP settings did not refresh live command registry: %#v", snapshot.Commands)
	}

	document := configruntime.ConfigDocumentFromTyped(application.CurrentConfig())
	document["command"].(map[string]any)["prefixes"] = []any{"!"}
	request(http.MethodPut, "/api/config", document)
	event := compositionCommandEvent("!say write plugin")
	application.HandleAdapterEvent(context.Background(), event)
	waitCompositionLog(t, logs, func(entry logging.Summary) bool {
		return entry.Details["fixture_event"] == "config.changed" && entry.Details["fixture_value"] == "plugin"
	})
	values := request(http.MethodGet, "/api/plugins/raylea.echo/settings", nil)["values"].(map[string]any)
	stored, err = repository.ReadAll(context.Background(), "raylea.echo")
	if err != nil || values["value"] != "plugin" || stored["value"] != "plugin" {
		t.Fatalf("action/HTTP/SQLite disagree: HTTP=%#v stored=%#v err=%v", values, stored, err)
	}

	request(http.MethodPost, "/api/governance/blacklist/entries", map[string]any{
		"entry_type": "user", "target_id": event.SenderID, "reason": "fixture",
		"scope": map[string]any{"kind": "global", "source_protocol": "onebot11", "source_adapter": "", "bot_id": ""},
	})
	event.EventID = "composition-blocked"
	event.PlainText = "!say write blocked"
	application.HandleAdapterEvent(context.Background(), event)
	waitCompositionLog(t, logs, func(entry logging.Summary) bool { return entry.Details["error_code"] == "permission.blacklisted" })
	stored, err = repository.ReadAll(context.Background(), "raylea.echo")
	if err != nil || stored["value"] != "plugin" {
		t.Fatalf("blocked event reached runtime: %#v %v", stored, err)
	}
	if err := application.Close(); err != nil {
		t.Fatal(err)
	}
	if snapshot, _ := application.Plugins().Get("raylea.echo"); snapshot.RuntimeState != string(plugins.RuntimeStateStopped) {
		t.Fatalf("shutdown retained running runtime: %#v", snapshot)
	}
}

func compositionCommandEvent(text string) chatevent.NormalizedEvent {
	return chatevent.NormalizedEvent{
		Kind: chatevent.EventKindMessage, EventID: "composition-command", SourceProtocol: "onebot11", SourceAdapter: "fixture-adapter",
		EventType: "message.group", Timestamp: time.Now().Unix(), BotID: "1001", ConversationType: "group", ConversationID: "2001",
		SenderID: "3001", ActorRole: "member", PlainText: text,
	}
}

func waitCompositionLog(t *testing.T, entries <-chan logging.Summary, match func(logging.Summary) bool) logging.Summary {
	t.Helper()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	var observed []string
	for {
		select {
		case entry := <-entries:
			if match(entry) {
				return entry
			}
			observed = append(observed, fmt.Sprintf("%s %#v", entry.Message, entry.Details))
		case <-timeout.C:
			t.Fatalf("composition effect not observed; logs=%v", observed)
		}
	}
}
