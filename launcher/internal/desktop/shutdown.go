package desktop

import (
	"errors"
	"time"
)

const shutdownGracePeriod = 4 * time.Second
const stopGracePeriod = 5 * time.Second
const processKillWait = 2 * time.Second
const processExitPoll = 50 * time.Millisecond

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
