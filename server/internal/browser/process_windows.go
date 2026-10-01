package browser

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var isProcessInJob = windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")

// A job owns descendants even when the browser's original process exits.
func startBrowserProcess(command *exec.Cmd) (func(context.Context) error, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	if err := command.Start(); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(command.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		_ = windows.CloseHandle(process)
	}
	if err == nil {
		err = resumeBrowserProcess(uint32(command.Process.Pid))
	}
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		_ = windows.CloseHandle(job)
		return nil, err
	}
	var mu sync.Mutex
	var pending []windows.Handle
	return func(ctx context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		if job == 0 {
			return nil
		}
		if err := stopBrowserJob(ctx, job, &pending); err != nil {
			return err
		}
		if err := windows.CloseHandle(job); err != nil {
			return fmt.Errorf("close browser job: %w", err)
		}
		job = 0
		return nil
	}, nil
}

// TerminateJobObject can report an empty job before process handles are signaled.
// Reap each batch before enumerating again so late descendants remain owned.
func stopBrowserJob(ctx context.Context, job windows.Handle, pending *[]windows.Handle) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("browser process tree did not exit before the cleanup deadline: %w", err)
		}
		for _, process := range *pending {
			if err := windows.TerminateProcess(process, 1); err != nil && !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
				return fmt.Errorf("terminate browser job process: %w", err)
			}
		}
		for len(*pending) > 0 {
			process := (*pending)[0]
			state, err := windows.WaitForSingleObject(process, 0)
			if err != nil {
				return fmt.Errorf("wait for browser job process: %w", err)
			}
			if state == windows.WAIT_OBJECT_0 {
				if err := windows.CloseHandle(process); err != nil {
					return fmt.Errorf("close browser process handle: %w", err)
				}
				*pending = (*pending)[1:]
				continue
			}
			select {
			case <-ctx.Done():
				return fmt.Errorf("browser process tree did not exit before the cleanup deadline: %w", ctx.Err())
			case <-ticker.C:
			}
		}
		ids, err := browserJobProcessIDs(ctx, job)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		for _, pid := range ids {
			process, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
			if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
				continue // The process exited before its handle could be opened.
			}
			if err != nil {
				return fmt.Errorf("open browser job process: %w", err)
			}
			var member int32
			ok, _, callErr := isProcessInJob.Call(uintptr(process), uintptr(job), uintptr(unsafe.Pointer(&member)))
			if ok == 0 || member == 0 {
				_ = windows.CloseHandle(process)
				if ok == 0 {
					return fmt.Errorf("check browser job membership: %w", callErr)
				}
				continue // A recycled PID must not target an unrelated process.
			}
			*pending = append(*pending, process)
		}
	}
}

func browserJobProcessIDs(ctx context.Context, job windows.Handle) ([]uintptr, error) {
	const headerWords = 8 / unsafe.Sizeof(uintptr(0))
	capacity := 32
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		buffer := make([]uintptr, int(headerWords)+capacity)
		header := (*struct{ assigned, count uint32 })(unsafe.Pointer(&buffer[0]))
		err := windows.QueryInformationJobObject(job, windows.JobObjectBasicProcessIdList, uintptr(unsafe.Pointer(&buffer[0])), uint32(len(buffer))*uint32(unsafe.Sizeof(buffer[0])), nil)
		if errors.Is(err, windows.ERROR_MORE_DATA) {
			capacity = max(capacity*2, int(header.assigned))
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("query browser job processes: %w", err)
		}
		return buffer[headerWords : headerWords+uintptr(header.count)], nil
	}
}

// Assign the suspended child before it can spawn or relaunch descendants.
func resumeBrowserProcess(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		_, err = windows.ResumeThread(thread)
		_ = windows.CloseHandle(thread)
		return err
	}
	return fmt.Errorf("browser primary thread is unavailable: %w", err)
}
