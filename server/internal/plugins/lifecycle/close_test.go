package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
)

func TestCloseCancelsAndJoinsAcceptedLifecycleWork(t *testing.T) {
	t.Parallel()
	cat := catalog.New([]plugins.Snapshot{{PluginID: "fixture", Valid: true, RegistrationState: "installed", DesiredState: "disabled"}})
	controller := newTestController(t, Deps{Plugins: cat})
	release, err := controller.acquireOperation(t.Context(), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	started, finished := make(chan struct{}), make(chan error, 1)
	if !controller.launch(func() {
		close(started)
		_, err := controller.acquireOperation(controller.lifecycleContext(), "fixture")
		finished <- err
	}) {
		t.Fatal("active lifecycle rejected work")
	}
	<-started
	closed := make(chan struct{})
	go func() { controller.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("close left queued lifecycle work blocked")
	}
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatalf("pending operation result: %v", err)
	}
	controller.Close()
	if controller.launch(func() { t.Error("work ran after close") }) {
		t.Fatal("closed lifecycle accepted new work")
	}
	if _, err := controller.Enable(t.Context(), "fixture"); !errors.Is(err, context.Canceled) {
		t.Fatalf("closed lifecycle accepted enable: %v", err)
	}
	if snapshot, _ := cat.Get("fixture"); snapshot.DesiredState != "disabled" {
		t.Fatal("rejected enable changed persisted intent")
	}
}
