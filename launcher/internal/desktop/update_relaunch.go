package desktop

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// WaitForPIDFlag makes a relaunched Launcher wait for the updating process to
// exit before the Wails single-instance lock is taken.
const WaitForPIDFlag = "--wait-for-pid"

// launcherExecutable follows the release package layout rather than
// os.Executable, which points at the moved-aside file after an update.
func launcherExecutable(basePath, goos string) string {
	switch goos {
	case "windows":
		return filepath.Join(basePath, "RayleaLauncher.exe")
	case "darwin":
		return filepath.Join(basePath, "RayleaLauncher.app", "Contents", "MacOS", "RayleaLauncher")
	default:
		return filepath.Join(basePath, "RayleaLauncher")
	}
}

func startDetachedLauncher(basePath, goos string, pid int) error {
	command := exec.Command(launcherExecutable(basePath, goos), WaitForPIDFlag, strconv.Itoa(pid))
	command.Dir = basePath
	configureDetachedProcess(command)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

// WaitForProcessExit returns once pid has exited or the relaunch budget passes.
func WaitForProcessExit(pid int) {
	deadline := time.Now().Add(relaunchWaitTimeout)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(processExitPoll)
	}
}
