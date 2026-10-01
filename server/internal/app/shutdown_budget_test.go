package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/render"
	"github.com/RayleaBot/RayleaBot/server/internal/storage"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
)

func TestClosePhasesFitReportedBudgetFromCurrentConfig(t *testing.T) {
	for _, grace := range []int{1, 10, 60, 600} {
		application := &App{state: &appRuntimeState{config: config.Config{Runtime: config.RuntimeConfig{ShutdownGraceSeconds: 1}}}}
		application.state.SetConfig(config.Config{Runtime: config.RuntimeConfig{ShutdownGraceSeconds: grace}})
		budgets := application.state.CurrentConfig().Runtime.ShutdownBudgets()
		application.requestShutdown(systemsvc.StopIntentStop)
		if application.process.shutdownBudgets != budgets {
			t.Fatalf("shutdown did not use latest configuration: %#v", application.process.shutdownBudgets)
		}
		phases := application.shutdownPhases(application.process.shutdownBudgets, func(context.Context) error { return nil })
		total := budgets.Announcement
		for _, phase := range phases {
			if phase.budget <= 0 {
				t.Fatalf("unbounded phase: %s", phase.name)
			}
			total += phase.budget
		}
		reported := time.Duration(budgets.TotalSeconds()) * time.Second
		if total > reported || reported-total >= time.Second {
			t.Fatalf("grace=%d: phases=%s reported=%s", grace, total, reported)
		}
	}
}

func TestCloseCutsOffHungTaskAndRenderAtTheirPhaseBudget(t *testing.T) {
	for _, kind := range []string{"task", "render"} {
		t.Run(kind, func(t *testing.T) {
			release := make(chan struct{})
			started := make(chan struct{})
			application := &App{}
			if kind == "task" {
				executor := tasks.NewExecutor(tasks.NewRegistry(), time.Minute)
				application.platform.TaskExecutor = executor
				_, err := executor.Submit("fixture", "fixture", func(ctx context.Context, _ tasks.ProgressReporter) (*tasks.ResultSummary, error) {
					close(started)
					<-release
					return nil, ctx.Err()
				})
				if err != nil {
					t.Fatal(err)
				}
				<-started
			} else {
				root := t.TempDir()
				store, err := storage.Open(filepath.Join(root, "state.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = store.Close() })
				renderer, err := render.NewService(render.Options{RepoRoot: root, Store: store, OutputRoot: filepath.Join(root, "render"), Runner: &hangingCloseRunner{started: started, release: release}})
				if err != nil {
					t.Fatal(err)
				}
				application.renderStack.Renderer = renderer
			}
			// Cleanup releases the deliberately uncooperative closer before waiting.
			defer close(release)
			budgets := (config.RuntimeConfig{}).ShutdownBudgets()
			budgets.Tasks, budgets.Browser = 40*time.Millisecond, 80*time.Millisecond
			name := "close task executor"
			if kind == "render" {
				name = "close browsers and render"
			}
			found := false
			for _, phase := range application.shutdownPhases(budgets, func(context.Context) error { return nil }) {
				if phase.name != name {
					continue
				}
				found = true
				before := time.Now()
				err, done := runShutdownPhase(phase.budget, phase.stop)
				elapsed := time.Since(before)
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("phase returned %v", err)
				}
				if elapsed < phase.budget || elapsed > phase.budget+time.Second {
					t.Fatalf("elapsed=%s budget=%s", elapsed, phase.budget)
				}
				select {
				case <-done:
					t.Fatal("hung closer reported complete")
				default:
				}
				t.Cleanup(func() {
					select {
					case <-done:
					case <-time.After(time.Second):
						t.Error("closer did not settle after release")
					}
				})
			}
			if !found {
				t.Fatalf("missing close phase %s", name)
			}
		})
	}
}

type hangingCloseRunner struct {
	started chan struct{}
	release <-chan struct{}
}

func (*hangingCloseRunner) Render(context.Context, render.Document) ([]byte, error) {
	return nil, errors.New("unexpected render")
}
func (r *hangingCloseRunner) Close() error { close(r.started); <-r.release; return nil }
