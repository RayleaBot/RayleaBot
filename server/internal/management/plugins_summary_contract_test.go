package management

import (
	"net/http/httptest"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestListPluginsReturnsUnifiedCommandShape(t *testing.T) {
	t.Parallel()
	snapshots := []plugins.Snapshot{
		{
			PluginID: "raylea.echo", Name: "Echo", Description: "Echo command", Valid: true,
			RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running",
			PackageSourceType: "catalog", PackagePublisherVerified: true,
			Commands:      []plugins.Command{{ID: "echo", Name: "echo", DisplayName: "Echo", TriggerType: "exact", TriggerNames: []string{"echo"}, Description: "Echo text", Usage: "/echo <text>", Permission: "everyone"}},
			CommandGroups: []plugins.CommandGroup{{ID: "basic", Title: "Basic", Commands: []string{"echo"}}},
			Help:          &plugins.Help{Title: "Echo", Summary: "Echo commands"},
		},
		{
			PluginID: "other", Name: "Other", Valid: true,
			RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running",
			PackageSourceType: "development",
			Commands:          []plugins.Command{{ID: "other-echo", Name: "echo", DisplayName: "Other echo", TriggerType: "setting", SettingsKey: "echo_command", Description: "Other echo", Usage: "/echo", Permission: "everyone"}},
		},
	}
	router := pluginRouter(t, plugincatalog.New(snapshots))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("GET", "/api/plugins", nil))
	if recorder.Code != 200 {
		t.Fatalf("status = %d", recorder.Code)
	}
	items := decodeBody(t, recorder.Body.Bytes())["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	var first map[string]any
	for _, item := range items {
		candidate := item.(map[string]any)
		if candidate["id"] == "raylea.echo" {
			first = candidate
			break
		}
	}
	if first == nil || first["role"] != "official" || len(first["command_conflicts"].([]any)) != 1 {
		t.Fatalf("summary = %#v", first)
	}
	command := first["commands"].([]any)[0].(map[string]any)
	if command["id"] != "echo" || command["trigger"].(map[string]any)["type"] != "exact" {
		t.Fatalf("command = %#v", command)
	}
	if _, exists := command["command_source"]; exists {
		t.Fatalf("legacy command source leaked: %#v", command)
	}
	groups := first["command_groups"].([]any)
	if len(groups) != 1 || groups[0].(map[string]any)["id"] != "basic" {
		t.Fatalf("command_groups = %#v", groups)
	}
	help := first["help"].(map[string]any)
	if help["title"] != "Echo" || help["summary"] != "Echo commands" {
		t.Fatalf("help = %#v", help)
	}
}
