package desktop

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"
)

func TestUpdateHandoffCancellationAndHelperOwnership(t *testing.T) {
	name := "raylea-server"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	build := exec.Command("go", "build", "-o", binary, filepath.Join("testdata", "stop_intent_server.go"))
	build.Env = replaceEnvironmentValues(os.Environ(), map[string]string{"GOWORK": "off"})
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, output)
	}
	payload, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	newCoordinator := func(t *testing.T) (*Coordinator, string) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, name), payload, 0755); err != nil {
			t.Fatal(err)
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address := listener.Addr().String()
		listener.Close()
		_, port, _ := net.SplitHostPort(address)
		writeTestFile(t, filepath.Join(root, "config", "user.yaml"), "server:\n  host: 127.0.0.1\n  port: "+port+"\n")
		c := NewCoordinator(root, "", 0, nil)
		c.settings = LauncherSettings{InstallationRoot: root, CloseBehavior: CloseAskEveryTime}
		c.initialized = true
		c.publishRelease(ReleaseCheckSnapshot{Status: ReleaseUpdateAvailable, UpdateAvailable: true, CanCheck: true})
		t.Setenv("RAYLEA_TEST_SERVICE_ADDR", "127.0.0.1:0")
		t.Setenv("RAYLEA_TEST_ADDR_FILE", filepath.Join(root, "address.txt"))
		t.Setenv("RAYLEA_TEST_INTENT_FILE", filepath.Join(root, "intent.txt"))
		t.Setenv("RAYLEA_TEST_APPLY_FILE", filepath.Join(root, "applied.txt"))
		t.Cleanup(func() {
			if err := c.process.ForceKill(); err != nil {
				t.Error(err)
			}
		})
		return c, root
	}
	startService := func(t *testing.T, c *Coordinator, root string) {
		if err := c.process.Start(ResolveLauncherSettings(c.settings)); err != nil {
			t.Fatal(err)
		}
		address := waitForTestFile(t, filepath.Join(root, "address.txt"))
		_, port, err := net.SplitHostPort(address)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, filepath.Join(root, "config", "user.yaml"), "server:\n  host: 127.0.0.1\n  port: "+port+"\n")
		if err := c.Refresh(); err != nil {
			t.Fatal(err)
		}
		if got := c.process.ShutdownWaitBudget(); got != 47*time.Second {
			t.Fatalf("polled budget = %s", got)
		}
	}
	for _, running := range []bool{false, true} {
		t.Run(fmt.Sprintf("resume only managed running=%v", running), func(t *testing.T) {
			c, root := newCoordinator(t)
			if running {
				startService(t, c, root)
			}
			relaunched := false
			c.relaunch = func(base, goos string, pid int, resume bool) error {
				if c.process.IsRunning() {
					t.Fatal("relaunched before service exited")
				}
				if base != root || goos != runtime.GOOS || pid != os.Getpid() || resume != running {
					t.Fatalf("relaunch(%s, %s, %d, %v)", base, goos, pid, resume)
				}
				if _, err := os.Stat(filepath.Join(root, "applied.txt")); err != nil {
					t.Fatal("relaunched before apply")
				}
				relaunched = true
				return nil
			}
			if !c.ApplyUpdate() || !relaunched {
				t.Fatal("update did not relaunch")
			}
			if running {
				intent := waitForTestFile(t, filepath.Join(root, "intent.txt"))
				if intent != "update" {
					t.Fatalf("intent = %s", intent)
				}
				if exit := c.Snapshot().Launcher.ProcessExit; exit == nil || exit.Kind != ExitPlanned {
					t.Fatalf("exit = %+v", exit)
				}
			}
		})
	}
	for _, failure := range []string{"apply", "relaunch"} {
		t.Run(failure+" failure leaves service stopped", func(t *testing.T) {
			c, root := newCoordinator(t)
			startService(t, c, root)
			if failure == "apply" {
				t.Setenv("RAYLEA_TEST_APPLY_FILE", "")
			}
			c.relaunch = func(string, string, int, bool) error {
				if failure == "apply" {
					t.Fatal("relaunched after apply failed")
				}
				return errors.New("relaunch fixture failed")
			}
			if c.ApplyUpdate() || c.process.IsRunning() {
				t.Fatal("failed update restarted the service")
			}
			release := c.Snapshot().Launcher.ReleaseCheck
			if release.Status != ReleaseFailed || release.ErrorCode != "launcher.update_"+failure+"_failed" || !release.CanCheck || !release.UpdateAvailable {
				t.Fatalf("release = %+v", release)
			}
		})
	}
	for _, confirm := range []bool{false, true} {
		t.Run(fmt.Sprintf("external confirmation=%v", confirm), func(t *testing.T) {
			c, root := newCoordinator(t)
			var stopCalls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/healthz":
					fmt.Fprint(w, `{"status":"ok"}`)
				case "/readyz":
					fmt.Fprint(w, `{"status":"ready"}`)
				case "/api/launcher/shutdown":
					stopCalls++
					w.WriteHeader(http.StatusForbidden)
				default:
					w.WriteHeader(http.StatusForbidden)
					fmt.Fprint(w, `{"error":{"code":"launcher.control_required","message":"fixture"}}`)
				}
			}))
			defer server.Close()
			_, port, _ := net.SplitHostPort(server.Listener.Addr().String())
			writeTestFile(t, filepath.Join(root, "config", "user.yaml"), "server:\n  host: 127.0.0.1\n  port: "+port+"\n")
			host := &webUIHost{testServiceHost: testServiceHost{confirmExternal: confirm}}
			c.host = host
			c.relaunch = func(string, string, int, bool) error { t.Fatal("external stop must not relaunch"); return nil }
			if c.ApplyUpdate() {
				t.Fatal("external service update succeeded")
			}
			if _, err := os.Stat(filepath.Join(root, "applied.txt")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("confirmation proceeded to apply")
			}
			snapshot := c.Snapshot()
			if snapshot.Launcher.ControlCapability != ControlOpenWeb || stopCalls != 0 {
				t.Fatal("external service exposed stop capability")
			}
			if !confirm && (snapshot.Launcher.ReleaseCheck.Status != ReleaseCancelled || snapshot.Launcher.ReleaseCheck.ErrorCode != "launcher.update_cancelled") {
				t.Fatalf("cancel = %+v", snapshot.Launcher.ReleaseCheck)
			}
			if confirm && host.openedURL != server.URL+"/" {
				t.Fatalf("Web URL = %s", host.openedURL)
			}
		})
	}
	t.Run("exit cancels and reaps download", func(t *testing.T) {
		c, root := newCoordinator(t)
		pidFile := filepath.Join(root, "helper-pid.txt")
		t.Setenv("RAYLEA_TEST_DOWNLOAD_PID_FILE", pidFile)
		c.relaunch = func(string, string, int, bool) error { t.Error("cancelled update relaunched"); return nil }
		done := make(chan bool, 1)
		go func() { done <- c.ApplyUpdate() }()
		t.Cleanup(func() { _ = c.Shutdown() })
		pid, err := strconv.Atoi(waitForTestFile(t, pidFile))
		if err != nil {
			t.Fatal(err)
		}
		if !processAlive(pid) {
			t.Fatal("download helper did not start")
		}
		if err := c.Shutdown(); err != nil {
			t.Fatal(err)
		}
		select {
		case result := <-done:
			if result {
				t.Fatal("cancelled update succeeded")
			}
		case <-time.After(time.Second):
			t.Fatal("shutdown returned before update completed")
		}
		if processAlive(pid) {
			t.Fatal("download helper survived shutdown")
		}
		if release := c.Snapshot().Launcher.ReleaseCheck; release.Status != ReleaseCancelled || release.ErrorCode != "launcher.update_cancelled" {
			t.Fatalf("deliberate cancellation reported as failure: %+v", release)
		}
		if c.ApplyUpdate() {
			t.Fatal("new update admitted after shutdown")
		}
		if _, err := os.Stat(filepath.Join(root, "applied.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("cancelled download proceeded to apply")
		}
	})
}

func waitForTestFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return string(data)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fixture did not write %s", filepath.Base(path))
	return ""
}
