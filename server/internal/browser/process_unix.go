//go:build !windows

package browser

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
)

func startBrowserProcess(command *exec.Cmd) (func(context.Context) error, error) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return nil, err
	}
	pid := command.Process.Pid
	var once sync.Once
	return func(context.Context) error {
		once.Do(func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })
		return nil
	}, nil
}
