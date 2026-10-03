package runtime

import (
	"golang.org/x/sys/windows"
	"testing"
)

func assertTreeProcessExited(t *testing.T, pid int) {
	t.Helper()
	process, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err == windows.ERROR_INVALID_PARAMETER {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = windows.CloseHandle(process) }()
	status, err := windows.WaitForSingleObject(process, 1000)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("grandchild %d survived: wait=%d err=%v", pid, status, err)
	}
}
