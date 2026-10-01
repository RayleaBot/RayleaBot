package app

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"time"
)

func TestRunSupervisorPreservesFirstFailure(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		criticalFirst bool
	}{
		{name: "background fails while HTTP is blocked"},
		{name: "HTTP fails before background", criticalFirst: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			supervisor := newRunSupervisor(t.Context())
			defer supervisor.Cancel()
			firstErr := errors.New("first failure")
			laterErr := errors.New("later failure")
			started := make(chan struct{}, 2)
			release := make(chan struct{})
			run := func(first bool) func(context.Context) error {
				return func(ctx context.Context) error {
					started <- struct{}{}
					if first {
						select {
						case <-release:
							return firstErr
						case <-ctx.Done():
							return ctx.Err()
						}
					}
					<-ctx.Done()
					return laterErr
				}
			}
			supervisor.GoCritical(run(testCase.criticalFirst))
			supervisor.Go(run(!testCase.criticalFirst))
			<-started
			<-started
			close(release)
			select {
			case <-supervisor.Context().Done():
			case <-time.After(3 * time.Second):
				t.Fatal("task failure did not cancel the blocked peer")
			}
			for range 2 {
				if err := supervisor.Wait(); !errors.Is(err, firstErr) || errors.Is(err, laterErr) {
					t.Fatalf("Wait() = %v, want first failure", err)
				}
			}
		})
	}
}

func TestRunSupervisorCleanHTTPExitDoesNotHideBackgroundFailure(t *testing.T) {
	supervisor := newRunSupervisor(t.Context())
	defer supervisor.Cancel()
	expected := errors.New("background failure during cancellation")
	supervisor.Go(func(ctx context.Context) error {
		<-ctx.Done()
		return expected
	})
	supervisor.GoCritical(func(context.Context) error { return nil })
	if err := supervisor.Wait(); !errors.Is(err, expected) {
		t.Fatalf("Wait() = %v, want background failure", err)
	}
}

func TestRunSupervisorCancellationDoesNotHideTaskFailure(t *testing.T) {
	for range 100 {
		ctx, cancel := context.WithCancel(t.Context())
		supervisor := newRunSupervisor(ctx)
		expected := errors.New("background failure")
		supervisor.Go(func(context.Context) error {
			cancel()
			return expected
		})
		<-supervisor.Context().Done()
		if err := supervisor.Wait(); !errors.Is(err, expected) {
			t.Fatalf("Wait() = %v, want failure racing cancellation", err)
		}
		supervisor.Cancel()
	}
}

func TestRunSupervisorNormalCancellationWaitsForWorkers(t *testing.T) {
	supervisor := newRunSupervisor(t.Context())
	stopped := make(chan struct{})
	supervisor.Go(func(ctx context.Context) error {
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	})
	supervisor.Cancel()
	if err := supervisor.Wait(); err != nil {
		t.Fatalf("Wait() = %v, want normal shutdown", err)
	}
	select {
	case <-stopped:
	default:
		t.Fatal("Wait returned before the worker stopped")
	}
}

func TestFirstShutdownRequestKeepsItsIntent(t *testing.T) {
	intents := []systemsvc.StopIntent{systemsvc.StopIntentStop, systemsvc.StopIntentRestart, systemsvc.StopIntentUpdate}
	for _, first := range intents {
		t.Run(string(first), func(t *testing.T) {
			application := &App{}
			entered, release := make(chan struct{}), make(chan struct{})
			var canceled atomic.Int32
			application.process.runCancel = func() { canceled.Add(1); close(entered); <-release }
			var workers sync.WaitGroup
			workers.Go(func() { application.requestShutdown(first) })
			<-entered
			for _, later := range intents {
				workers.Go(func() { application.requestShutdown(later) })
			}
			close(release)
			workers.Wait()
			if got := application.process.shutdownIntent.Load(); got == nil || *got != first {
				t.Fatalf("shutdown intent = %v, want %q", got, first)
			}
			if canceled.Load() != 1 {
				t.Fatalf("shutdown ran %d times", canceled.Load())
			}
		})
	}
}
