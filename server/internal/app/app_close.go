package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	systemsvc "github.com/RayleaBot/RayleaBot/server/internal/operations/system"
	"github.com/RayleaBot/RayleaBot/server/internal/platform/errorcodes"
)

func (a *App) Close() error {
	if a == nil {
		return nil
	}
	a.process.closeOnce.Do(func() { a.process.closeErr = a.closeResources() })
	return a.process.closeErr
}

type shutdownPhase struct {
	name   string
	budget time.Duration
	stop   func(context.Context) error
}

// The caller stops waiting at the phase deadline even if a closer ignores
// cancellation. done tracks its actual completion for dependent resources.
func runShutdownPhase(budget time.Duration, stop func(context.Context) error) (error, <-chan struct{}) {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	result := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		result <- stop(ctx)
	}()
	select {
	case err := <-result:
		return err, done
	case <-ctx.Done():
		return ctx.Err(), done
	}
}

func (a *App) logShutdownError(phase string, err error) {
	logger := slog.Default()
	if a.state != nil && a.state.Logger != nil {
		logger = a.state.Logger
	}
	logger.Error("服务关闭阶段清理失败", "component", "app", "phase", phase,
		"error_code", errorcodes.PlatformInternalError, "err", err)
}

// closeOnce protects the plan and its resource owners; timed-out closers may
// still finish while later independent phases run.
func (a *App) closeResources() error {
	a.requestShutdown(systemsvc.StopIntentStop)
	errs := []error{a.process.announcementErr}
	completed := []<-chan struct{}{a.process.announcementDone}
	phases := a.shutdownPhases(a.process.shutdownBudgets, func(ctx context.Context) error {
		// Never close the database or release the config lock beneath a closer
		// that is still using them. Process exit releases them if a worker hangs.
		for _, done := range completed {
			select {
			case <-done:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})
	for i, phase := range phases {
		err, done := runShutdownPhase(phase.budget, phase.stop)
		if i < len(phases)-1 {
			completed = append(completed, done)
		}
		if err != nil {
			a.logShutdownError(phase.name, err)
			errs = append(errs, fmt.Errorf("%s: %w", phase.name, err))
		}
	}
	return errors.Join(errs...)
}

func (a *App) shutdownPhases(b config.ShutdownBudgets, awaitClosers func(context.Context) error) []shutdownPhase {
	return []shutdownPhase{
		{"shutdown http server", b.HTTP, a.shutdownHTTPServer},
		{"stop background workers", b.Workers, func(context.Context) error { return a.stopWorkers() }},
		{"drain accepted events", b.DispatchDrain, a.drainEvents},
		{"stop runtime managers", b.PluginGrace + b.KillWait, func(ctx context.Context) error {
			// Reserve the final kill wait for runtime process reclamation.
			deadline, _ := ctx.Deadline()
			stopCtx, cancel := context.WithDeadline(ctx, deadline.Add(-b.KillWait))
			defer cancel()
			return a.stopRuntimeManagers(stopCtx)
		}},
		{"stop adapters", b.Adapters, func(ctx context.Context) error {
			err := a.stopAdapter(ctx)
			if a.eventStack.Conversations != nil {
				a.eventStack.Conversations.Close()
			}
			return err
		}},
		{"close task executor", b.Tasks, func(context.Context) error {
			if a.platform.TaskExecutor != nil {
				return a.platform.TaskExecutor.Close()
			}
			return nil
		}},
		{"close browsers and render", b.Browser, func(context.Context) error {
			return closeConcurrently(func() error {
				if a.services.Browser != nil {
					return a.services.Browser.CloseAll()
				}
				return nil
			}, func() error {
				if a.renderStack.Renderer != nil {
					return a.renderStack.Renderer.Close()
				}
				return nil
			})
		}},
		{"flush persistence", b.Finalize, func(ctx context.Context) error {
			if err := awaitClosers(ctx); err != nil {
				return err
			}
			return a.closePersistence()
		}},
	}
}

func closeConcurrently(closers ...func() error) error {
	errs := make([]error, len(closers))
	var workers sync.WaitGroup
	for i, close := range closers {
		workers.Go(func() { errs[i] = close() })
	}
	workers.Wait()
	return errors.Join(errs...)
}

func (a *App) stopWorkers() error {
	a.process.runCancelMu.Lock()
	wait := a.process.runWait
	a.process.runCancelMu.Unlock()
	return closeConcurrently(func() error {
		if wait != nil {
			return wait()
		}
		return nil
	}, func() error {
		if a.platform.Scheduler != nil {
			a.platform.Scheduler.Stop()
		}
		return nil
	}, func() error {
		if a.pluginStack.PluginInstaller != nil {
			return a.pluginStack.PluginInstaller.Close()
		}
		return nil
	}, func() error {
		if a.pluginStack.PluginUninstaller != nil {
			return a.pluginStack.PluginUninstaller.Close()
		}
		return nil
	}, func() error {
		if a.services.PluginLifecycle != nil {
			a.services.PluginLifecycle.Close()
		}
		return nil
	})
}

func (a *App) closePersistence() error {
	var errs []error
	if a.platform.Tasks != nil {
		errs = append(errs, a.platform.Tasks.Close())
	}
	if a.platform.Logs != nil {
		a.platform.Logs.Close()
	}
	if a.platform.Storage != nil {
		errs = append(errs, a.platform.Storage.Close())
	}
	if a.configLifecycleLock != nil {
		errs = append(errs, a.configLifecycleLock.Close())
	}
	return errors.Join(errs...)
}

func (a *App) drainEvents(ctx context.Context) error {
	var err error
	if a.services.EventIngress != nil {
		err = a.services.EventIngress.Drain(ctx)
	}
	if a.eventStack.Dispatcher != nil {
		err = errors.Join(err, a.eventStack.Dispatcher.DrainAll(ctx))
	}
	return err
}

func (a *App) stopRuntimeManagers(ctx context.Context) error {
	if a.eventStack.Dispatcher != nil {
		defer a.eventStack.Dispatcher.Close()
	}
	if a.runtimes == nil {
		return nil
	}
	stopErr := a.runtimes.StopAll(ctx)
	var projectionErr error
	if a.pluginStack.Plugins != nil {
		for _, plugin := range a.pluginStack.Plugins.List() {
			manager, exists := a.runtimes.Get(plugin.PluginID)
			if !exists {
				continue
			}
			// A timeout does not prove the process has exited.
			snapshot := manager.Snapshot()
			_, err := a.pluginStack.Plugins.SetRuntimeResult(plugin.PluginID, string(snapshot.State), snapshot.LastErrorCode, snapshot.LastErrorMessage)
			projectionErr = errors.Join(projectionErr, err)
		}
	}
	return errors.Join(stopErr, projectionErr)
}

func (a *App) stopAdapter(ctx context.Context) error {
	if a.services.Protocol == nil {
		return nil
	}
	return a.services.Protocol.Stop(ctx)
}

func (a *App) shutdownHTTPServer(ctx context.Context) error {
	if a.process.server == nil {
		return nil
	}
	if err := a.process.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return errors.Join(err, a.process.server.Close())
	}
	return nil
}
