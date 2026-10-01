//go:build !windows

package runtime

import (
	"os/exec"
	"syscall"
)

func startPluginProcess(command *exec.Cmd) (func(), error) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return nil, err
	}
	pid := command.Process.Pid
	return func() { _ = syscall.Kill(-pid, syscall.SIGKILL) }, nil
}
