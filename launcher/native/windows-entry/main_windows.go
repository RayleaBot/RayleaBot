package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	launcherRuntimeDirectory = "launcher"
	launcherExecutable       = "RayleaLauncher.exe"
	launcherEntryPIDEnv      = "RAYLEA_LAUNCHER_ENTRY_PID"
)

func showLaunchError(err error) {
	message, messageErr := windows.UTF16PtrFromString(
		"RayleaBot 启动器无法启动。\r\n\r\n" + err.Error() +
			"\r\n\r\n请确认发布包已完整解压，并保留 launcher 文件夹。",
	)
	title, titleErr := windows.UTF16PtrFromString("RayleaBot 启动失败")
	if messageErr == nil && titleErr == nil {
		_, _ = windows.MessageBox(0, message, title, windows.MB_OK|windows.MB_ICONERROR|windows.MB_SETFOREGROUND)
	}
}

func createSupervisionJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("创建进程监督任务失败: %w", err)
	}
	information := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	information.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&information)),
		uint32(unsafe.Sizeof(information)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return 0, fmt.Errorf("配置进程监督任务失败: %w", err)
	}
	return job, nil
}

func assignProcessToJob(job windows.Handle, processID int) error {
	process, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false,
		uint32(processID),
	)
	if err != nil {
		return fmt.Errorf("打开 Electron 进程失败: %w", err)
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		return fmt.Errorf("监督 Electron 进程失败: %w", err)
	}
	return nil
}

func run() error {
	entryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("读取启动入口路径失败: %w", err)
	}
	installRoot := filepath.Dir(entryPath)
	target := filepath.Join(installRoot, launcherRuntimeDirectory, launcherExecutable)
	targetInfo, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("未找到 Electron 启动程序 %q: %w", target, err)
	}
	if !targetInfo.Mode().IsRegular() {
		return fmt.Errorf("Electron 启动程序不是普通文件: %q", target)
	}

	job, err := createSupervisionJob()
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)

	command := exec.Command(target, os.Args[1:]...)
	command.Dir = installRoot
	command.Env = append(
		os.Environ(),
		launcherEntryPIDEnv+"="+strconv.Itoa(os.Getpid()),
	)
	if err := command.Start(); err != nil {
		return fmt.Errorf("启动 Electron 应用 %q 失败: %w", target, err)
	}
	if err := assignProcessToJob(job, command.Process.Pid); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return err
	}
	return command.Wait()
}

func main() {
	if err := run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			os.Exit(exitError.ExitCode())
		}
		showLaunchError(err)
		os.Exit(1)
	}
}
