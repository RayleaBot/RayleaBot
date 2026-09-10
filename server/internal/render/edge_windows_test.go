//go:build windows

package render

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/systeminfo"
	"github.com/chromedp/chromedp"
	"github.com/coder/websocket"
)

func edgeTestRunner(t *testing.T) (*chromiumRunner, string) {
	t.Helper()
	executable := filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe")
	if _, err := os.Stat(executable); err != nil {
		t.Skipf("system Edge is unavailable: %v", err)
	}
	root := t.TempDir()
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)
	if filepath.Clean(os.TempDir()) != filepath.Clean(root) {
		t.Fatal("test cannot isolate its browser profiles")
	}
	// A caller must not be able to disable the flag required to retain Edge's
	// actual browser process and its DevTools output pipe.
	runner := NewChromiumRunner(ChromiumOptions{
		BrowserPath: executable,
		BrowserArgs: []string{"--edge-skip-compat-layer-relaunch=false"},
	})
	t.Cleanup(func() {
		closeDone := make(chan error, 1)
		go func() { closeDone <- runner.Close() }()
		select {
		case err := <-closeDone:
			if err != nil {
				t.Errorf("cleanup Edge runner: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("Edge runner cleanup exceeded its deadline")
		}
		// The regression being tested could detach Edge before chromedp owns
		// it. These files are exclusively beneath this test's private TMP.
		// This fallback is after the assertions, so it cannot make a leak pass.
		closeEdgeTestProfiles(t, root)
	})
	return runner, root
}

func TestSystemEdgeRenderRetainsBrowserProcessAndCloses(t *testing.T) {
	runner, root := edgeTestRunner(t)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	content, err := runner.Render(ctx, Document{
		Width: 80, Height: 48, Output: "png",
		HTML: "<!doctype html><html><body style='margin:0;background:#247'>Edge ownership</body></html>",
	})
	if err != nil {
		t.Fatalf("render with system Edge: %v", err)
	}
	image, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("decode screenshot: %v", err)
	}
	if image.Bounds().Dx() != 80 || image.Bounds().Dy() != 48 {
		t.Fatalf("unexpected screenshot bounds: %v", image.Bounds())
	}
	browser := chromedp.FromContext(runner.browserCtx).Browser
	process := browser.Process()
	if process == nil {
		t.Fatal("allocator has no browser process")
	}
	infos, err := systeminfo.GetProcessInfo().Do(cdp.WithExecutor(ctx, browser))
	if err != nil {
		t.Fatalf("read actual browser processes: %v", err)
	}
	browserPID := int64(0)
	for _, info := range infos {
		if info.Type == "browser" {
			browserPID = info.ID
		}
	}
	if browserPID != int64(process.Pid) {
		t.Fatalf("Edge relaunched outside allocator ownership: allocator=%d actual=%d", process.Pid, browserPID)
	}
	profile := runner.profileDir
	if filepath.Dir(profile) != root {
		t.Fatalf("unexpected owned profile: %q", profile)
	}
	if _, err := os.Stat(profile); err != nil {
		t.Fatalf("owned profile missing before Close: %v", err)
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("close Edge: %v", err)
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("repeat close Edge: %v", err)
	}
	if err := process.Signal(syscall.Signal(0)); !errors.Is(err, syscall.EINVAL) {
		t.Errorf("allocator process %d was not reaped: %v", process.Pid, err)
	}
	if _, err := os.Stat(profile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("owned Edge profile survived Close: %v", err)
	}
}

func TestSystemEdgeStartupCancellationAndRepeatedCloseAreBounded(t *testing.T) {
	for _, timeout := range []time.Duration{5 * time.Millisecond, 25 * time.Millisecond, 100 * time.Millisecond} {
		t.Run(timeout.String(), func(t *testing.T) {
			runner, root := edgeTestRunner(t)
			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, renderErr := runner.Render(ctx, Document{
					Width: 32, Height: 32, Output: "png",
					HTML: "<!doctype html><html><body>cancel startup</body></html>",
				})
				if !errors.Is(renderErr, context.DeadlineExceeded) {
					done <- fmt.Errorf("render returned %v, want request deadline", renderErr)
					return
				}
				done <- errors.Join(runner.Close(), runner.Close())
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("startup cancellation or repeated Close did not finish")
			}
			profiles, err := filepath.Glob(filepath.Join(root, "rayleabot-chromium-*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(profiles) != 0 {
				t.Errorf("cancelled startup retained profiles: %v", profiles)
			}
		})
	}
}

// A failed startup may leave the real browser outside the allocator. Use only
// the DevTools endpoint published under this test's private profile to close it.
func closeEdgeTestProfiles(t *testing.T, root string) {
	t.Helper()
	if t.Failed() {
		// A detached compatibility launcher can report EOF before its child
		// creates DevToolsActivePort. Keep the private directory available
		// briefly so that this failure cannot strand that later child.
		deadline := time.Now().Add(3 * time.Second)
		for {
			ports, err := filepath.Glob(filepath.Join(root, "rayleabot-chromium-*", "DevToolsActivePort"))
			if err != nil || len(ports) > 0 || !time.Now().Before(deadline) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	profiles, err := os.ReadDir(root)
	if err != nil {
		t.Errorf("read owned profile root: %v", err)
		return
	}
	for _, entry := range profiles {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "rayleabot-chromium-") {
			continue
		}
		profile := filepath.Join(root, entry.Name())
		address, err := os.ReadFile(filepath.Join(profile, "DevToolsActivePort"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Errorf("read owned Edge endpoint: %v", err)
			continue
		}
		lines := strings.Fields(string(address))
		if len(lines) != 2 || !strings.HasPrefix(lines[1], "/devtools/browser/") {
			t.Errorf("invalid owned Edge endpoint: %q", address)
			continue
		}
		port, err := strconv.Atoi(lines[0])
		if err != nil || port < 1 || port > 65535 {
			t.Errorf("invalid Edge port: %q", lines[0])
			continue
		}
		endpoint := (&url.URL{Scheme: "ws", Host: fmt.Sprintf("127.0.0.1:%d", port), Path: lines[1]}).String()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		conn, _, err := websocket.Dial(ctx, endpoint, nil)
		if err == nil {
			body, _ := json.Marshal(map[string]any{"id": 1, "method": "Browser.close"})
			if err := conn.Write(ctx, websocket.MessageText, body); err != nil {
				t.Errorf("close owned Edge: %v", err)
			}
			_, _, _ = conn.Read(ctx)
			_ = conn.CloseNow()
		}
		cancel()
		deadline := time.Now().Add(3 * time.Second)
		for {
			err := os.RemoveAll(profile)
			if err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Errorf("owned Edge profile remains after fallback close: %v", err)
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
