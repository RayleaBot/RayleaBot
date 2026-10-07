//go:build !windows

package processoutput

import "os/exec"

func newCommand(executable string, args []string) *exec.Cmd { return exec.Command(executable, args...) }
