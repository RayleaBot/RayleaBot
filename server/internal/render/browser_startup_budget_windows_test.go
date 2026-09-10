//go:build windows

package render

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"golang.org/x/sys/windows"
)

const browserBudgetHelperEnvironment = "RAYLEABOT_RENDER_BUDGET_HELPER_URL"

type browserBudgetProcess struct {
	PID     int
	Profile string
}

func TestMain(m *testing.M) {
	// The test executable stands in for Chromium. Handle its allocator flags
	// before testing parses them; normal test processes keep their usual flow.
	if endpoint := os.Getenv(browserBudgetHelperEnvironment); endpoint != "" {
		os.Exit(runBrowserBudgetHelper(endpoint))
	}
	os.Exit(m.Run())
}

func runBrowserBudgetHelper(endpoint string) int {
	profile := ""
	for _, argument := range os.Args[1:] {
		if value, ok := strings.CutPrefix(argument, "--user-data-dir="); ok {
			profile = value
		}
	}
	if profile == "" {
		return 2
	}
	payload, _ := json.Marshal(browserBudgetProcess{PID: os.Getpid(), Profile: profile})
	client := &http.Client{Transport: &http.Transport{}, Timeout: time.Minute}
	ready, err := client.Post(endpoint+"/ready", "application/json", bytes.NewReader(payload))
	if err != nil {
		return 3
	}
	_ = ready.Body.Close()
	release, err := client.Get(endpoint + "/release")
	if err != nil {
		return 4
	}
	_ = release.Body.Close()
	if _, err := fmt.Fprintln(os.Stdout, "DevTools listening on "+strings.Replace(endpoint, "http://", "ws://", 1)+"/cdp"); err != nil {
		return 5
	}
	// Remain a live process until the runner cancels its allocator after the
	// deliberately rejected handshake. The server never responds to /hold.
	hold, err := client.Get(endpoint + "/hold")
	if err == nil {
		_ = hold.Body.Close()
	}
	return 6
}

func TestChromiumStartupUsesCallerBudgetBeyondTwentySeconds(t *testing.T) {
	ready := make(chan browserBudgetProcess, 1)
	release := make(chan struct{})
	holding := make(chan struct{})
	handshake := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/ready":
			var process browserBudgetProcess
			if err := json.NewDecoder(request.Body).Decode(&process); err != nil {
				http.Error(w, "invalid helper identity", http.StatusBadRequest)
				return
			}
			ready <- process
		case "/release":
			select {
			case <-release:
			case <-request.Context().Done():
			}
		case "/hold":
			close(holding)
			<-request.Context().Done()
		case "/cdp":
			select {
			case <-holding:
				close(handshake)
				w.WriteHeader(http.StatusTeapot)
			case <-request.Context().Done():
			}
		default:
			http.NotFound(w, request)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv(browserBudgetHelperEnvironment, server.URL)
	runner := NewChromiumRunner(ChromiumOptions{BrowserPath: os.Args[0]})
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	t.Cleanup(func() {
		cancel()
		if err := runner.Close(); err != nil {
			t.Errorf("cleanup delayed browser helper: %v", err)
		}
	})
	rendered := make(chan error, 1)
	go func() {
		_, err := runner.Render(ctx, Document{})
		rendered <- err
	}()
	var process browserBudgetProcess
	select {
	case process = <-ready:
	case err := <-rendered:
		t.Fatalf("helper did not start: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(process.PID))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	if !strings.HasPrefix(filepath.Base(process.Profile), "rayleabot-chromium-") {
		t.Fatalf("helper did not receive an owned profile: %q", process.Profile)
	}

	// This boundary is intentional: it must exceed chromedp's default 20s
	// URL wait while remaining inside the caller's unchanged 45s deadline.
	delay := time.NewTimer(21 * time.Second)
	defer delay.Stop()
	select {
	case err := <-rendered:
		t.Fatalf("startup ended before the caller budget: %v", err)
	case <-delay.C:
		close(release)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case err := <-rendered:
		var status ws.StatusError
		if !errors.As(err, &status) || status != ws.StatusError(http.StatusTeapot) {
			t.Fatalf("expected the delayed handshake rejection, got %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-handshake:
	default:
		t.Fatal("caller budget did not reach the delayed DevTools handshake")
	}
	if ctx.Err() != nil {
		t.Fatalf("caller budget was exhausted: %v", ctx.Err())
	}
	if status, err := windows.WaitForSingleObject(handle, 0); err != nil || status != windows.WAIT_OBJECT_0 {
		t.Errorf("failed startup left helper %d alive: status=%d error=%v", process.PID, status, err)
	}
	if _, err := os.Stat(process.Profile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("failed startup retained profile: %v", err)
	}
}
