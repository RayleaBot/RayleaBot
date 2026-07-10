//go:build !windows

package deps

import (
	"errors"
	"syscall"
)

func processIsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
