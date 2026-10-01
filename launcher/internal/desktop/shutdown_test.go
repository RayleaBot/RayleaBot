package desktop

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

type shutdownFixture struct {
	running, remains bool
	kills            int
	err              error
}

type gracefulShutdownProcess struct {
	running atomic.Bool
	killed  bool
}

func (p *gracefulShutdownProcess) IsRunning() bool { return p.running.Load() }
func (p *gracefulShutdownProcess) ForceKill() error {
	p.killed = true
	p.running.Store(false)
	return nil
}

func TestExitWaitsForServerCleanupWithoutDelayingCompletedShutdown(t *testing.T) {
	for _, cleanup := range []time.Duration{0, 5 * time.Second, 60 * time.Second} {
		t.Run(cleanup.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				process := &gracefulShutdownProcess{}
				process.running.Store(true)
				started := time.Now()
				err := stopManagedProcess(process, func() error {
					if cleanup == 0 {
						process.running.Store(false)
						return nil
					}
					ctx, finish := context.WithTimeout(t.Context(), cleanup)
					go func() {
						defer finish()
						<-ctx.Done()
						process.running.Store(false)
					}()
					return nil
				}, shutdownWaitBudget(60))
				if err != nil || process.killed {
					t.Fatalf("server cleanup was interrupted: killed=%v, err=%v", process.killed, err)
				}
				if elapsed := time.Since(started); elapsed > cleanup+processExitPoll {
					t.Fatalf("waited %s for %s cleanup", elapsed, cleanup)
				}
			})
		})
	}
}

func TestExitStillKillsServerAfterGraceDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		process := &gracefulShutdownProcess{}
		process.running.Store(true)
		started := time.Now()
		if err := stopManagedProcess(process, func() error { return nil }, shutdownGracePeriod); err != nil {
			t.Fatal(err)
		}
		if !process.killed || time.Since(started) != shutdownGracePeriod {
			t.Fatalf("kill fallback: killed=%v, elapsed=%s", process.killed, time.Since(started))
		}
	})
}

func (p *shutdownFixture) IsRunning() bool  { return p.running }
func (p *shutdownFixture) ForceKill() error { p.kills++; p.running = p.remains; return p.err }

func TestShutdownUsesFinalProcessState(t *testing.T) {
	gracefulFailure := errors.New("graceful fixture failure")
	killFailure := errors.New("kill fixture failure")
	for _, c := range []struct {
		name                 string
		running, remains     bool
		gracefulErr, killErr error
		wantFailure          bool
	}{
		{name: "already stopped"},
		{name: "graceful rejected then force stopped", running: true, gracefulErr: gracefulFailure},
		{name: "kill command error but process exited", running: true, killErr: killFailure},
		{name: "process remains after both attempts", running: true, remains: true, gracefulErr: gracefulFailure, killErr: killFailure, wantFailure: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			process := &shutdownFixture{running: c.running, remains: c.remains, err: c.killErr}
			err := stopManagedProcess(process, func() error { return c.gracefulErr }, 0)
			var boundary *BoundaryError
			if c.wantFailure {
				if !errors.As(err, &boundary) || boundary.Code != "launcher.shutdown_failed" || !errors.Is(err, gracefulFailure) || !errors.Is(err, killFailure) {
					t.Fatalf("failure lost: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !c.wantFailure {
				before := process.kills
				if err := stopManagedProcess(process, func() error { t.Fatal("repeated shutdown sent another request"); return nil }, 0); err != nil || process.kills != before {
					t.Fatalf("non-idempotent shutdown: %v", err)
				}
			}
		})
	}
}
