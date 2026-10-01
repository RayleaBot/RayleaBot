package desktop

import (
	"errors"
	"time"
)

// Older servers do not report a budget. New servers include every cleanup
// phase; allow a small margin for request handling and process teardown.
const shutdownGracePeriod = 30 * time.Second
const shutdownGraceMargin = 2 * time.Second
const processKillWait = 2 * time.Second
const processExitPoll = 50 * time.Millisecond

func shutdownWaitBudget(seconds int64) time.Duration {
	if seconds < 1 {
		return shutdownGracePeriod
	}
	const maximum = time.Duration(1<<63 - 1)
	if seconds > int64((maximum-shutdownGraceMargin)/time.Second) {
		return maximum
	}
	return time.Duration(seconds)*time.Second + shutdownGraceMargin
}

type managedProcessStopper interface {
	IsRunning() bool
	ForceKill() error
}

func stopManagedProcess(process managedProcessStopper, graceful func() error, grace time.Duration) error {
	if !process.IsRunning() {
		return nil
	}
	var gracefulErr error
	if graceful != nil {
		gracefulErr = graceful()
	}
	deadline := time.Now().Add(grace)
	for process.IsRunning() && time.Now().Before(deadline) {
		time.Sleep(processExitPoll)
	}
	if !process.IsRunning() {
		return nil
	}
	killErr := process.ForceKill()
	if !process.IsRunning() {
		return nil
	}
	return errors.Join(&BoundaryError{Code: "launcher.shutdown_failed", Message: "启动器退出失败：服务进程仍在运行。"}, gracefulErr, killErr)
}
