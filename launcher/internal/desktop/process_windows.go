//go:build windows

package desktop

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func configureChildProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

func configureDetachedProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000 | 0x00000008 | 0x00000200}
}

func superviseProcess(process *os.Process) (func(), error) {
	if process == nil {
		return nil, errors.New("服务进程不可用")
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("创建进程监督任务: %w", err)
	}
	closeJob := func() {
		_ = windows.CloseHandle(job)
	}
	information := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	information.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&information)),
		uint32(unsafe.Sizeof(information)),
	); err != nil {
		closeJob()
		return nil, fmt.Errorf("配置进程监督任务: %w", err)
	}
	processHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(process.Pid),
	)
	if err != nil {
		closeJob()
		return nil, fmt.Errorf("打开服务进程: %w", err)
	}
	defer windows.CloseHandle(processHandle)
	if err := windows.AssignProcessToJobObject(job, processHandle); err != nil {
		closeJob()
		return nil, fmt.Errorf("监督服务进程: %w", err)
	}
	return closeJob, nil
}

func processExitReason(state *os.ProcessState) string {
	if state == nil || state.ExitCode() < 0 {
		return ""
	}
	return fmt.Sprintf("退出码 %d", state.ExitCode())
}

func terminateProcessTree(pid int) error {
	command := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	configureChildProcess(command)
	output, err := command.CombinedOutput()
	if err != nil && !bytes.Contains(bytes.ToLower(output), []byte("not found")) {
		return fmt.Errorf("终止进程树: %w", err)
	}
	return nil
}

func findListeningProcess(port int) (int, string, error) {
	command := exec.Command("netstat", "-ano", "-p", "tcp")
	configureChildProcess(command)
	output, err := command.Output()
	if err != nil {
		return 0, "", err
	}
	pid := 0
	for _, raw := range strings.Split(string(output), "\n") {
		fields := strings.Fields(strings.TrimSpace(raw))
		if len(fields) < 5 || strings.ToUpper(fields[0]) != "TCP" || strings.ToUpper(fields[3]) != "LISTENING" {
			continue
		}
		addressPort, parseErr := strconv.Atoi(fields[1][strings.LastIndex(fields[1], ":")+1:])
		if parseErr == nil && addressPort == port {
			pid, _ = strconv.Atoi(fields[len(fields)-1])
			break
		}
	}
	if pid == 0 {
		return 0, "", nil
	}
	command = exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	configureChildProcess(command)
	output, err = command.Output()
	if err != nil {
		return pid, "", nil
	}
	line := strings.TrimSpace(strings.Split(string(output), "\n")[0])
	name := ""
	if strings.HasPrefix(line, "\"") {
		if end := strings.Index(line[1:], "\""); end >= 0 {
			name = line[1 : end+1]
		}
	}
	return pid, name, nil
}

func stopEndpointProcess(endpoint ServerEndpoint) (bool, error) {
	pid, name, err := findListeningProcess(endpoint.Port)
	if err != nil || pid == 0 {
		return false, err
	}
	if !strings.EqualFold(name, "raylea-server.exe") && !strings.EqualFold(name, "raylea-server") {
		return false, nil
	}
	if err := terminateProcessTree(pid); err != nil && !errors.Is(err, syscall.ESRCH) {
		return false, err
	}
	return true, nil
}
