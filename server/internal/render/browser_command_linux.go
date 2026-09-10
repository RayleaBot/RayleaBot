//go:build linux

package render

import (
	"os"
	"os/exec"
	"syscall"
)

// Preserve chromedp's Linux process ownership policy when observing its command.
func prepareBrowserCommand(command *exec.Cmd) {
	if _, lambda := os.LookupEnv("LAMBDA_TASK_ROOT"); lambda {
		return
	}
	if command.SysProcAttr == nil {
		command.SysProcAttr = new(syscall.SysProcAttr)
	}
	command.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
