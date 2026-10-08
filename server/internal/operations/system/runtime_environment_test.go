package system

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
	plugincatalog "github.com/RayleaBot/RayleaBot/server/internal/plugins/catalog"
	"github.com/RayleaBot/RayleaBot/server/internal/tasks"
	"github.com/RayleaBot/RayleaBot/server/tests/testutil"
)

func TestRuntimeBootstrapRefreshesPluginEnvironmentAfterFFmpegPreparation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, kind                 string
		prepareFails, refreshFails bool
		wantCalls                  int32
	}{
		{name: "ffmpeg", kind: "ffmpeg", wantCalls: 1},
		{name: "chromium", kind: "chromium"},
		{name: "preparation_failure", kind: "ffmpeg", prepareFails: true},
		{name: "reload_submission_failure", kind: "ffmpeg", refreshFails: true, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			registry := tasks.NewRegistry()
			executor := tasks.NewExecutor(registry, 5*time.Second)
			t.Cleanup(func() { _ = executor.Close() })
			var calls atomic.Int32
			var service *Service
			var err error
			service, err = New(Deps{
				CurrentConfig:  func() config.Config { return config.Config{} },
				CurrentSummary: func() config.Summary { return config.Summary{} },
				Plugins:        plugincatalog.New(nil), TaskExecutor: executor,
				PrepareRuntime: func(context.Context, string, string, deps.PrepareProgressReporter) (*deps.PrepareReport, error) {
					if tc.prepareFails {
						return nil, errors.New("preparation failed")
					}
					return &deps.PrepareReport{Kind: tc.kind, UsedPreparedStore: true}, nil
				},
				RefreshPluginTools: func(context.Context) error {
					calls.Add(1)
					state, _ := service.startupRuntimeState("ffmpeg")
					if state.Phase != StartupRuntimePhaseReady {
						return errors.New("FFmpeg is not ready")
					}
					if tc.refreshFails {
						return errors.New("reload submission failed")
					}
					return nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			id, err := service.SubmitRuntimeBootstrapTask([]string{tc.kind})
			if err != nil {
				t.Fatal(err)
			}
			want := tasks.StatusSucceeded
			if tc.prepareFails || tc.refreshFails {
				want = tasks.StatusFailed
			}
			testutil.WaitTask(t, registry, id, want)
			if got := calls.Load(); got != tc.wantCalls {
				t.Fatalf("refresh calls = %d, want %d", got, tc.wantCalls)
			}
			if tc.refreshFails {
				state, _ := service.startupRuntimeState("ffmpeg")
				if state.Phase != StartupRuntimePhaseReady {
					t.Fatal("reload failure invalidated prepared resource")
				}
			}
		})
	}
}
