//go:build !windows

package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func configureChildProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func configureDetachedProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func superviseProcess(*os.Process) (func(), error) {
	return func() {}, nil
}

func processExitReason(state *os.ProcessState) string {
	if state == nil {
		return ""
	}
	if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return "信号 " + status.Signal().String()
	}
	if state.ExitCode() >= 0 {
		return "退出码 " + strconv.Itoa(state.ExitCode())
	}
	return ""
}

func terminateProcessTree(pid int) error {
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

func findListeningProcess(port int) (int, string, error) {
	output, err := exec.Command("lsof", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		return 0, "", nil
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(strings.Split(string(output), "\n")[0]))
	if pid == 0 {
		return 0, "", nil
	}
	command, _ := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if command == "" {
		output, _ = exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
		command = strings.TrimSpace(string(output))
	}
	return pid, filepath.Base(command), nil
}

func stopEndpointProcess(endpoint ServerEndpoint) (bool, error) {
	pid, name, err := findListeningProcess(endpoint.Port)
	if err != nil || pid == 0 {
		return false, err
	}
	if name != "raylea-server" && name != "raylea-server.exe" {
		return false, nil
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return false, err
	}
	return true, nil
}
