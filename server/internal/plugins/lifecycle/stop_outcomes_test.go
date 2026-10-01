package lifecycle

import (
	"context"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	pluginruntime "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/settings"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestStopAnnouncesAndProjectsGracefulOrForcedExit(t *testing.T) {
	for _, tc := range []struct {
		name         string
		delay, grace time.Duration
		fail         bool
	}{
		{"configured grace exceeds five seconds", 6 * time.Second, 8 * time.Second, false},
		{"forced exit", time.Second, 100 * time.Millisecond, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			cat := catalog.New([]plugins.Snapshot{{PluginID: "fixture", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: "running"}})
			runtimes := pluginruntime.NewRegistry(logger, pluginruntime.Options{})
			controller := newTestController(t, Deps{Plugins: cat, Runtimes: runtimes, Logger: logger})
			manager := runtimes.GetOrCreate("fixture")
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			if err := manager.Start(t.Context(), pluginruntime.Spec{PluginID: "fixture", Command: executable, Args: []string{"-test.run=^TestLifecycleShutdownProcess$"},
				Env:         []string{"RAYLEABOT_LIFECYCLE_SHUTDOWN_FIXTURE=1", "RAYLEABOT_LIFECYCLE_SHUTDOWN_DELAY=" + tc.delay.String(), "GORACE=atexit_sleep_ms=0"},
				InitTimeout: 3 * time.Second, EventTimeout: time.Second, ShutdownGrace: tc.grace, EffectiveConcurrency: 1}, pluginruntime.InitPayload{Timezone: "Asia/Shanghai", CommandPrefixes: []string{"/"}}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = manager.Stop(ctx)
			})
			updates, unsubscribe := cat.Subscribe(8)
			defer unsubscribe()
			done := make(chan error, 1)
			go func() { done <- controller.StopAndResetPluginWithContext(t.Context(), "fixture") }()
			select {
			case state := <-updates:
				if state.RuntimeState != "stopping" {
					t.Fatalf("first state %s", state.RuntimeState)
				}
			case <-time.After(time.Second):
				t.Fatal("missing stopping announcement")
			}
			err = <-done
			if (err != nil) != tc.fail {
				t.Fatalf("stop error=%v", err)
			}
			actual, _ := cat.Get("fixture")
			if actual.RuntimeState != "stopped" {
				t.Fatalf("catalog not stopped: %#v", actual)
			}
			if tc.fail && actual.RuntimeErrorCode != errorcodes.PluginShutdownTimeout {
				t.Fatalf("lost force-kill diagnosis: %#v", actual)
			}
			if tc.fail {
				state, diagnosis := plugins.ProjectState(actual)
				if state != plugins.PluginStateFailed || diagnosis == nil || diagnosis.Kind != plugins.StateDiagnosisShutdownFailed {
					t.Fatalf("force-kill projection: %s %#v", state, diagnosis)
				}
			}
			if !tc.fail && actual.RuntimeErrorCode != "" {
				t.Fatalf("graceful stop diagnosed failure: %#v", actual)
			}
		})
	}
}
func TestReloadCancellationAndCleanupFailureClassification(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		shutdown bool
		want     tasks.Status
	}{
		{"operator", context.Canceled, false, tasks.StatusCancelled},
		{"shutdown", context.Canceled, true, tasks.StatusInterrupted},
		{"deadline", context.DeadlineExceeded, false, tasks.StatusFailed},
		{"cleanup failure", errors.Join(context.Canceled, errors.New("cleanup failed")), false, tasks.StatusFailed},
		{"committed settings", &settings.ApplyError{Stage: "notify", Cause: context.Canceled}, false, tasks.StatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := tasks.NewRegistry()
			controller := newTestController(t, Deps{Tasks: registry})
			id, _ := registry.Create("plugin.reload", "fixture")
			if tc.shutdown {
				controller.lifecycleCancel()
			}
			controller.failReloadTaskForError(id, "fixture", tc.err, "重载失败")
			snapshot, _ := registry.Get(id)
			if snapshot.Status != tc.want || snapshot.FinishedAt == nil {
				t.Fatalf("task %#v", snapshot)
			}
			if tc.want != tasks.StatusFailed && snapshot.Error != nil {
				t.Fatalf("cancellation has failure: %#v", snapshot.Error)
			}
		})
	}
}
func TestUninstallCancellationBeforeMutationIsCancelled(t *testing.T) {
	registry := tasks.NewRegistry()
	id, _ := registry.Create("plugin.uninstall", "fixture")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	service := &UninstallService{registry: registry, operations: NewOperationGate(), cancels: map[string]context.CancelFunc{}, deps: uninstallerDeps{now: time.Now}}
	service.execute(uninstallJob{taskID: id, pluginID: "fixture", ctx: ctx})
	snapshot, _ := registry.Get(id)
	if snapshot.Status != tasks.StatusCancelled || snapshot.Error != nil || snapshot.FinishedAt == nil {
		t.Fatalf("task %#v", snapshot)
	}
	for _, state := range []string{"partial", "committed", "rollback_failed"} {
		failure := &operationError{state: state}
		failure.add("remove", context.Canceled)
		if cancellationOnly(failure) {
			t.Fatalf("%s incorrectly cancelled", state)
		}
	}
}
func TestManagementActionDoesNotStartUnavailablePlugin(t *testing.T) {
	for _, state := range []string{"stopped", "starting", "stopping", "crashed", "backoff", "dead_letter"} {
		t.Run(state, func(t *testing.T) {
			cat := catalog.New([]plugins.Snapshot{{PluginID: "fixture", Valid: true, RegistrationState: "installed", DesiredState: "enabled", RuntimeState: state}})
			controller := newTestController(t, Deps{Plugins: cat})
			_, err := controller.InvokeManagementAction(t.Context(), "fixture", "snapshot", nil)
			var unavailable *plugins.NotRunningError
			if !errors.As(err, &unavailable) {
				t.Fatalf("error=%v", err)
			}
			if _, exists := controller.runtimes.Get("fixture"); exists {
				t.Fatal("management action started runtime")
			}
		})
	}
}
