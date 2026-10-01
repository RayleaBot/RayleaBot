package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Each helper inherits the parent's job/group, including the grandchild.
func TestPluginProcessTreeHelper(t *testing.T) {
	role := os.Getenv("RAYLEA_TREE_ROLE")
	if role == "" {
		return
	}
	marker := os.Getenv("RAYLEA_TREE_MARKER")
	if role == "grandchild" {
		for {
			_ = os.WriteFile(marker, []byte(fmt.Sprintf("%d %d", os.Getpid(), time.Now().UnixNano())), 0600)
			time.Sleep(20 * time.Millisecond)
		}
	}
	next := "grandchild"
	if role == "parent" {
		next = "child"
	}
	command := exec.Command(os.Args[0], "-test.run=^TestPluginProcessTreeHelper$")
	command.Env = append(os.Environ(), "RAYLEA_TREE_ROLE="+next)
	if err := command.Start(); err != nil {
		os.Exit(3)
	}
	if role == "child" {
		_ = command.Wait()
		os.Exit(0)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			os.Exit(4)
		}
		time.Sleep(10 * time.Millisecond)
	}
	decoder := json.NewDecoder(os.Stdin)
	for {
		var frame map[string]any
		if err := decoder.Decode(&frame); err != nil {
			time.Sleep(time.Hour)
			continue
		}
		switch frame["type"] {
		case "init":
			_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"type": "init_ack", "request_id": frame["request_id"], "status": "ready"})
		case "shutdown":
			if os.Getenv("RAYLEA_TREE_EXIT") == "1" {
				os.Exit(0)
			}
			time.Sleep(time.Hour)
		}
	}
}

func TestPluginStopReapsGrandchild(t *testing.T) {
	for _, graceful := range []bool{false, true} {
		t.Run(fmt.Sprint(graceful), func(t *testing.T) {
			t.Parallel()
			marker := filepath.Join(t.TempDir(), "grandchild")
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			manager := testManager()
			spec := Spec{PluginID: "fixture", Command: executable, Args: []string{"-test.run=^TestPluginProcessTreeHelper$"},
				Env:         []string{"RAYLEA_TREE_ROLE=parent", "RAYLEA_TREE_MARKER=" + marker, "GORACE=atexit_sleep_ms=0"},
				InitTimeout: 5 * time.Second, EventTimeout: time.Second, ShutdownGrace: 150 * time.Millisecond, EffectiveConcurrency: 1}
			if graceful {
				spec.Env = append(spec.Env, "RAYLEA_TREE_EXIT=1")
			}
			if err := manager.Start(t.Context(), spec, testInitPayload()); err != nil {
				t.Fatal(err)
			}
			handle := manager.proc
			t.Cleanup(func() { handle.killTree(); <-handle.Done() })
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			err = manager.Stop(ctx)
			if graceful && err != nil {
				t.Fatal(err)
			}
			if !graceful {
				assertRuntimeErrorCode(t, err, codePluginShutdownTimeout)
			}
			if manager.Snapshot().State != StateStopped {
				t.Fatalf("state: %#v", manager.Snapshot())
			}
			time.Sleep(100 * time.Millisecond)
			before, err := os.ReadFile(marker)
			if err != nil {
				t.Fatal(err)
			}
			var pid int
			if _, err = fmt.Sscanf(string(before), "%d", &pid); err != nil {
				t.Fatal(err)
			}
			assertTreeProcessExited(t, pid)
			time.Sleep(150 * time.Millisecond)
			after, err := os.ReadFile(marker)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatal("grandchild continued running after plugin stop")
			}
		})
	}
}
