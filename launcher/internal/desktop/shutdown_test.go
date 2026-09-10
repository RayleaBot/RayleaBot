package desktop

import (
	"errors"
	"testing"
)

type shutdownFixture struct {
	running, remains bool
	kills            int
	err              error
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
