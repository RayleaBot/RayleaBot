package lifecycle

import (
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestAcceptedEnablePublishesInitializationFailure(t *testing.T) {
	t.Parallel()
	cat := catalog.New([]plugins.Snapshot{{PluginID: "broken", Valid: true, RegistrationState: "installed", DesiredState: "disabled", ManifestPath: "missing/info.json"}})
	controller := newTestController(t, Deps{Plugins: cat, RepoRoot: t.TempDir()})
	if _, err := controller.Enable(t.Context(), "broken"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snapshot, _ := cat.Get("broken")
		state, diagnosis := plugins.ProjectState(snapshot)
		if state == plugins.PluginStateFailed {
			if diagnosis == nil || diagnosis.Kind != plugins.StateDiagnosisInitializationFailed || diagnosis.LastErrorCode != errorcodes.PluginArtifactInvalid {
				t.Fatalf("failure projection: %#v", diagnosis)
			}
			if _, err := controller.Disable(t.Context(), "broken"); err != nil {
				t.Fatal(err)
			}
			snapshot, _ = cat.Get("broken")
			state, _ = plugins.ProjectState(snapshot)
			if state != plugins.PluginStateDisabled {
				t.Fatalf("disabled failed plugin stayed %s", state)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("initialization failure remained enabled or starting in management state")
}
