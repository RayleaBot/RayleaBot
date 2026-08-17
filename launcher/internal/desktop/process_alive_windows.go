//go:build windows

package desktop

import (
	"fmt"
	"os/exec"
	"strings"
)

func processAlive(pid int) bool {
	command := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	configureChildProcess(command)
	output, err := command.Output()
	return err == nil && strings.HasPrefix(strings.TrimSpace(string(output)), "\"")
}
