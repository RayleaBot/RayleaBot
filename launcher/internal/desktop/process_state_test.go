package desktop

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

func TestProcessExitFixture(t *testing.T) {
	if value := os.Getenv("RAYLEA_EXIT_TEST_CODE"); value != "" {
		code, _ := strconv.Atoi(value)
		os.Exit(code)
	}
}

func TestProcessExitRecordsPlanAndCodeWithoutReadingLogs(t *testing.T) {
	for _, planned := range []bool{false, true} {
		for _, code := range []int{0, 7} {
			t.Run(fmt.Sprintf("planned=%v/code=%d", planned, code), func(t *testing.T) {
				command := exec.Command(os.Args[0], "-test.run=^TestProcessExitFixture$")
				command.Env = replaceEnvironmentValues(os.Environ(), map[string]string{"RAYLEA_EXIT_TEST_CODE": strconv.Itoa(code)})
				configureChildProcess(command)
				if err := command.Start(); err != nil {
					t.Fatal(err)
				}
				p := NewProcessController("")
				p.cmd = command
				if planned {
					p.MarkStopping()
				}
				p.wait(command)
				running, stopping, exit := p.State()
				kind := ExitUnexpected
				if planned {
					kind = ExitPlanned
				}
				if running || stopping || exit == nil || exit.Kind != kind || exit.ExitCode != code {
					t.Fatalf("state = %v, %v, %+v", running, stopping, exit)
				}
				c := NewCoordinator(t.TempDir(), "", 0, nil)
				c.process = p
				snapshot := c.buildSnapshot(operationContext{}, EnvironmentInspection{}, snapshotOptions{processOwnership: OwnershipNone})
				c.publish(snapshot)
				if planned && snapshot.Launcher.LastLocalError != "" {
					t.Fatal("planned exit reported as a crash")
				}
				if !planned && snapshot.Launcher.LastLocalError == "" {
					t.Fatal("unexpected exit hidden")
				}
				wantTray := "异常退出"
				if planned {
					wantTray = "已停止"
				}
				if got := trayState(snapshot).TrayStatusSummary; got != wantTray {
					t.Fatalf("tray = %s", got)
				}
				snapshot.Launcher.ProcessExit.ExitCode = 99
				if c.Snapshot().Launcher.ProcessExit.ExitCode != code {
					t.Fatal("exit snapshot aliases mutable state")
				}
			})
		}
	}
}

func TestObservedShutdownSurvivesReadinessAndHealthFailures(t *testing.T) {
	for _, readiness := range []string{"ready", "setup_required", "failed", "unreadable"} {
		t.Run(readiness, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/healthz":
					fmt.Fprint(w, `{"status":"ok"}`)
				case "/readyz":
					if readiness == "unreadable" {
						w.WriteHeader(500)
						return
					}
					fmt.Fprintf(w, `{"status":%q}`, readiness)
				case "/api/launcher/status":
					fmt.Fprint(w, `{"status":"shutting_down","shutdown_budget_seconds":80}`)
				}
			}))
			defer server.Close()
			c := NewCoordinator(t.TempDir(), "", 0, nil)
			c.process.cmd = &exec.Cmd{}
			operation := operationContext{endpoint: ServerEndpoint{BaseURL: server.URL + "/"}}
			if err := c.refreshWithInspection(operation, EnvironmentInspection{}); err != nil {
				t.Fatal(err)
			}
			if got := c.process.ShutdownWaitBudget(); got != 82*time.Second {
				t.Fatalf("budget = %s", got)
			}
			server.Close()
			for range 2 {
				if err := c.refreshWithInspection(operation, EnvironmentInspection{}); err != nil {
					t.Fatal(err)
				}
				snapshot := c.Snapshot()
				if snapshot.Launcher.ProcessLifecycle != Stopping || snapshot.Launcher.LastLocalError != "" || trayState(snapshot).TrayStatusSummary != "停止中" {
					t.Fatalf("lost known stop: %+v", snapshot.Launcher)
				}
				if got := c.process.ShutdownWaitBudget(); got != 82*time.Second {
					t.Fatalf("lost last successful budget: %s", got)
				}
			}
		})
	}
}

func TestShutdownBudgetUsesLatestSuccessfulStatusAndOlderServerFallback(t *testing.T) {
	p := NewProcessController("")
	p.cmd = &exec.Cmd{}
	for _, tc := range []struct {
		seconds int64
		want    time.Duration
	}{
		{0, 30 * time.Second}, {1, 3 * time.Second}, {45, 47 * time.Second}, {120, 122 * time.Second}, {-1, 30 * time.Second}, {0, 30 * time.Second}, {1<<63 - 1, time.Duration(1<<63 - 1)},
	} {
		p.RememberShutdownBudget(tc.seconds)
		if got := p.ShutdownWaitBudget(); got != tc.want {
			t.Fatalf("budget(%d) = %s, want %s", tc.seconds, got, tc.want)
		}
	}
}
