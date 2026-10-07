package events

import (
	"reflect"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestPluginStateEventKeepsReceivedStateWithCurrentDisplayConflicts(t *testing.T) {
	t.Parallel()
	registry := catalog.New([]plugins.Snapshot{
		{PluginID: "a", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running", Commands: []plugins.Command{{ID: "echo", Name: "echo", DisplayName: "Echo", TriggerNames: []string{"echo"}}}, DefaultConfig: map[string]any{"key": "value"}},
		{PluginID: "b", Valid: true, RegistrationState: "installed", DesiredState: "disabled", Commands: []plugins.Command{{ID: "echo", Name: "echo", DisplayName: "Echo"}}},
	})
	updates, unsubscribe := registry.Subscribe(4)
	defer unsubscribe()
	if _, err := registry.SetRuntimeState("a", "stopping"); err != nil {
		t.Fatal(err)
	}
	received := <-updates
	if _, err := registry.SetRuntimeState("a", "running"); err != nil {
		t.Fatal(err)
	}
	current := pluginSnapshotsForConflicts(registry)
	payload := pluginStateEventFrame(received, current).Data.(PluginStatePayload)
	if payload.State != "stopping" || payload.PluginID != "a" || len(payload.Commands) != 1 || !reflect.DeepEqual(payload.CommandConflicts, []string{"echo"}) {
		t.Fatalf("received event projection changed: %+v", payload)
	}
	if received.DefaultConfig["key"] != "value" || current[0].DefaultConfig != nil || current[0].RuntimeState != "running" {
		t.Fatal("event payload was replaced or complete publication lost its declarations")
	}
	payload.Commands[0].Trigger.Names[0] = "mutated"
	if got, _ := registry.Get("a"); !reflect.DeepEqual(got.Commands[0].TriggerNames, []string{"echo"}) {
		t.Fatal("event projection exposed catalog command storage")
	}
}
