package processoutput

import (
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func newCommand(executable string, args []string) *exec.Cmd {
	if !strings.EqualFold(filepath.Ext(executable), ".cmd") && !strings.EqualFold(filepath.Ext(executable), ".bat") {
		command := exec.Command(executable, args...)
		command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}
		return command
	}
	// cmd /s strips one outer quote pair; preserve the quoted shim path inside it.
	parts := []string{syscall.EscapeArg(executable)}
	for _, arg := range args {
		parts = append(parts, syscall.EscapeArg(arg))
	}
	command := exec.Command("cmd.exe")
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000, CmdLine: `cmd.exe /d /s /c "` + strings.Join(parts, " ") + `"`}
	return command
}
