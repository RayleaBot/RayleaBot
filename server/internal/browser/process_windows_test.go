package browser

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestBrowserProcessTreeHelper(t *testing.T) {
	switch os.Getenv("RAYLEA_BROWSER_TREE_ROLE") {
	case "child":
		time.Sleep(time.Minute)
		os.Exit(0)
	case "parent":
		count, err := strconv.Atoi(os.Getenv("RAYLEA_BROWSER_TREE_COUNT"))
		if err != nil {
			os.Exit(2)
		}
		var pids []string
		for range count {
			child := exec.Command(os.Args[0], "-test.run=^TestBrowserProcessTreeHelper$")
			child.Env = append(os.Environ(), "RAYLEA_BROWSER_TREE_ROLE=child")
			if err := child.Start(); err != nil {
				os.Exit(3)
			}
			pids = append(pids, strconv.Itoa(child.Process.Pid))
		}
		if err := os.WriteFile(os.Getenv("RAYLEA_BROWSER_TREE_PID"), []byte(strings.Join(pids, " ")), 0600); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
}

func TestBrowserStopWaitsForDescendantsAfterParentExit(t *testing.T) {
	for _, test := range []struct {
		name    string
		count   int
		expired bool
	}{
		{"parent exited", 1, false},
		{"grow process list", 40, false},
		{"retry after deadline", 1, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "child.pid")
			command := exec.Command(os.Args[0], "-test.run=^TestBrowserProcessTreeHelper$")
			command.Env = append(os.Environ(), "RAYLEA_BROWSER_TREE_ROLE=parent", "RAYLEA_BROWSER_TREE_PID="+marker, "RAYLEA_BROWSER_TREE_COUNT="+strconv.Itoa(test.count))
			stop, err := startBrowserProcess(command)
			if err != nil {
				t.Fatal(err)
			}
			exited := make(chan struct{})
			var waitErr error
			go func() { waitErr = command.Wait(); close(exited) }()
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), browserShutdownTimeout)
				defer cancel()
				if err := stop(ctx); err != nil {
					t.Error(err)
				}
				select {
				case <-exited:
				case <-ctx.Done():
					t.Error("helper parent was not reaped")
				}
			})
			select {
			case <-exited:
				if waitErr != nil {
					t.Fatal(waitErr)
				}
			case <-time.After(browserShutdownTimeout):
				t.Fatal("helper parent did not exit")
			}
			rawPID, err := os.ReadFile(marker)
			if err != nil {
				t.Fatal(err)
			}
			var children []windows.Handle
			for _, raw := range strings.Fields(string(rawPID)) {
				pid, err := strconv.ParseUint(raw, 10, 32)
				if err != nil {
					t.Fatal(err)
				}
				child, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = windows.CloseHandle(child) }()
				children = append(children, child)
				if state, err := windows.WaitForSingleObject(child, 0); state != uint32(windows.WAIT_TIMEOUT) || err != nil {
					t.Fatalf("descendant did not outlive its parent: state=%d err=%v", state, err)
				}
			}
			if len(children) != test.count {
				t.Fatalf("descendants = %d, want %d", len(children), test.count)
			}
			if test.expired {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				defer cancel()
				if err := stop(ctx); !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expired cleanup = %v", err)
				}
			}
			ctx, cancel := context.WithTimeout(t.Context(), browserShutdownTimeout)
			defer cancel()
			if err := stop(ctx); err != nil {
				t.Fatal(err)
			}
			for _, child := range children {
				if state, err := windows.WaitForSingleObject(child, 0); state != windows.WAIT_OBJECT_0 || err != nil {
					t.Fatalf("cleanup returned before descendant exit: state=%d err=%v", state, err)
				}
			}
			if err := stop(ctx); err != nil {
				t.Fatalf("repeated cleanup: %v", err)
			}
		})
	}
}
