package processoutput

import (
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func newCommand(executable string, args []string) *exec.Cmd {
	if !strings.EqualFold(filepath.Ext(executable), ".cmd") && !strings.EqualFold(filepath.Ext(executable), ".bat") {
		return exec.Command(executable, args...)
	}
	// cmd /s strips one outer quote pair; preserve the quoted shim path inside it.
	parts := []string{syscall.EscapeArg(executable)}
	for _, arg := range args {
		parts = append(parts, syscall.EscapeArg(arg))
	}
	command := exec.Command("cmd.exe")
	command.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /d /s /c "` + strings.Join(parts, " ") + `"`}
	return command
}
