package desktop

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestServiceOperationsSendShutdownIntent(t *testing.T) {
	serverName := "raylea-server"
	if runtime.GOOS == "windows" {
		serverName += ".exe"
	}
	binary := filepath.Join(t.TempDir(), serverName)
	build := exec.Command("go", "build", "-o", binary, filepath.Join("testdata", "stop_intent_server.go"))
	build.Env = replaceEnvironmentValues(os.Environ(), map[string]string{"GOWORK": "off"})
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build test service: %v\n%s", err, output)
	}
	payload, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"stop", "restart", "update", "exit", "restart blocked by exit"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, serverName), payload, 0755); err != nil {
				t.Fatal(err)
			}
			intentFile := filepath.Join(root, "intent.txt")
			addressFile := filepath.Join(root, "address.txt")
			t.Setenv("RAYLEA_TEST_SERVICE_ADDR", "127.0.0.1:0")
			t.Setenv("RAYLEA_TEST_ADDR_FILE", addressFile)
			t.Setenv("RAYLEA_TEST_INTENT_FILE", intentFile)
			writeTestFile(t, filepath.Join(root, "config", "user.yaml"), "server:\n  host: 127.0.0.1\n  port: 8080\n")
			host := &testServiceHost{}
			service := NewService(root, "", 0, host)
			coordinator := service.coordinator
			coordinator.settings = LauncherSettings{InstallationRoot: root, CloseBehavior: CloseAskEveryTime}
			coordinator.initialized = true
			t.Cleanup(func() {
				if err := coordinator.process.ForceKill(); err != nil {
					t.Error(err)
				}
			})
			if err := coordinator.process.Start(ResolveLauncherSettings(coordinator.settings)); err != nil {
				t.Fatal(err)
			}
			// Let the child own its listener before any client probes can reuse a
			// released ephemeral port. Restart subsequently keeps this same address.
			deadline := time.Now().Add(5 * time.Second)
			var address []byte
			for {
				address, err = os.ReadFile(addressFile)
				if err == nil && len(address) > 0 {
					break
				}
				if !coordinator.process.IsRunning() || time.Now().After(deadline) {
					t.Fatalf("test service did not bind its address: %v, diagnostics: %v", err, coordinator.process.RecentStderr())
				}
				time.Sleep(10 * time.Millisecond)
			}
			_, port, err := net.SplitHostPort(string(address))
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("RAYLEA_TEST_SERVICE_ADDR", string(address))
			writeTestFile(t, filepath.Join(root, "config", "user.yaml"), "server:\n  host: 127.0.0.1\n  port: "+port+"\n")
			if err := coordinator.Refresh(); err != nil {
				t.Fatal(err)
			}
			coordinator.process.mu.RLock()
			firstPID := coordinator.process.cmd.Process.Pid
			coordinator.process.mu.RUnlock()
			wantIntent := action
			switch action {
			case "stop":
				err = service.Stop()
			case "restart":
				err = service.Restart()
			case "exit":
				wantIntent = "stop"
				err = service.ServiceShutdown()
			case "restart blocked by exit":
				wantIntent = "restart"
				coordinator.startups.block(true)
				if err := service.Restart(); !errors.Is(err, errStartupBlocked) {
					t.Fatalf("Restart() = %v, want startup blocked", err)
				}
			case "update":
				coordinator.publishRelease(ReleaseCheckSnapshot{Status: ReleaseUpdateAvailable, UpdateAvailable: true})
				if coordinator.ApplyUpdate() {
					t.Fatal("update unexpectedly relaunched the desktop")
				}
				if got := coordinator.Snapshot().Launcher.ReleaseCheck.ErrorCode; got != "launcher.update_apply_failed" {
					t.Fatalf("update failure = %q", got)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			intent, err := os.ReadFile(intentFile)
			if err != nil || string(intent) != wantIntent {
				t.Fatalf("shutdown intent = %q, want %q, err=%v", intent, wantIntent, err)
			}
			if action == "restart" {
				coordinator.process.mu.RLock()
				second := coordinator.process.cmd
				coordinator.process.mu.RUnlock()
				if second == nil || second.Process.Pid == firstPID {
					t.Fatal("restart did not replace the managed process")
				}
				operation, err := coordinator.operationContext()
				if err != nil || !coordinator.quickHealthy(operation.endpoint) {
					t.Fatalf("restarted service is not healthy: %v", err)
				}
			} else if coordinator.process.IsRunning() {
				t.Fatal("service still running after stop")
			}
		})
	}
}

func TestRestartRejectsUnmanagedService(t *testing.T) {
	service := NewService(t.TempDir(), "", 0, &testServiceHost{})
	if err := service.Restart(); err == nil {
		t.Fatal("restart accepted an unmanaged service")
	}
}
