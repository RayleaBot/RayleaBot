package processoutput

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestChild(t *testing.T) {
	switch os.Getenv("R6_PROCESS_CHILD") {
	case "streams":
		os.Stdout.WriteString("中文输出")
		os.Stderr.WriteString("失败")
		os.Exit(7)
	case "stdout":
		os.Stdout.Write([]byte{255})
		os.Exit(0)
	case "stderr":
		os.Stderr.Write([]byte{255})
		os.Exit(0)
	}
}
func TestCapture(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("R6_PROCESS_CHILD", "streams")
	result, err := Run([]string{exe, "-test.run=^TestChild$"}, "")
	if err != nil || result.Code != 7 || result.Stdout != "中文输出" || result.Stderr != "失败" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, stream := range []string{"stdout", "stderr"} {
		t.Setenv("R6_PROCESS_CHILD", stream)
		if _, err := Run([]string{exe, "-test.run=^TestChild$"}, ""); err == nil {
			t.Fatalf("accepted invalid UTF-8 in %s", stream)
		}
	}
	result, err = Run([]string{filepath.Join(t.TempDir(), "missing")}, "")
	if err != nil || result.Code != 127 {
		t.Fatalf("missing executable: %+v %v", result, err)
	}
}
func TestWindowsShimWithSpaces(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows command shim")
	}
	path := filepath.Join(t.TempDir(), "node shim.cmd")
	if err := os.WriteFile(path, []byte("@echo off\r\necho %~1\r\nexit /b 7\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := Run([]string{path, "two words"}, "")
	if err != nil || result.Code != 7 || result.Stdout != "two words\r\n" {
		t.Fatalf("shim: %+v %v", result, err)
	}
}
