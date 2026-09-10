package render

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"

	"github.com/RayleaBot/RayleaBot/server/internal/platform/deps"
)

func newTestChromiumRunner(t *testing.T) *chromiumRunner {
	t.Helper()
	repoRoot := filepath.Join("..", "..", "..")
	browserPath, err := deps.NewManager(repoRoot).ResolvePreparedEntrypoint("chromium", "browser")
	if err != nil {
		t.Skipf("managed chromium is not prepared: %v", err)
	}

	runner := NewChromiumRunner(ChromiumOptions{BrowserPath: browserPath})
	t.Cleanup(func() {
		if err := runner.Close(); err != nil {
			t.Errorf("close test Chromium runner: %v", err)
		}
	})
	return runner
}

func TestChromiumRunnerCleanupStopsBrowser(t *testing.T) {
	for _, cancelRequest := range []bool{false, true} {
		name := "completed_request"
		if cancelRequest {
			name = "cancelled_request"
		}
		t.Run(name, func(t *testing.T) {
			var runner *chromiumRunner
			var process *os.Process
			var profileDir string
			var browserCtx context.Context
			t.Cleanup(func() {
				if runner != nil {
					if err := runner.Close(); err != nil {
						t.Errorf("fallback browser cleanup: %v", err)
					}
				}
			})
			t.Run("owner", func(t *testing.T) {
				runner = newTestChromiumRunner(t)
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				content, err := runner.Render(ctx, Document{
					Width: 32, Height: 32, Output: "png",
					HTML: "<!doctype html><html><body>cleanup</body></html>",
				})
				if err != nil {
					t.Fatalf("render before cleanup: %v", err)
				}
				if len(content) == 0 {
					t.Fatal("expected screenshot content")
				}
				browserCtx = runner.browserCtx
				browser := chromedp.FromContext(browserCtx).Browser
				process = browser.Process()
				arguments, err := cdpbrowser.GetBrowserCommandLine().Do(cdp.WithExecutor(ctx, browser))
				if err != nil {
					t.Fatalf("read browser command line: %v", err)
				}
				for _, argument := range arguments {
					if value, ok := strings.CutPrefix(argument, "--user-data-dir="); ok {
						profileDir = value
						break
					}
				}
				if profileDir == "" {
					t.Fatal("browser command line has no temporary profile")
				}
				if _, err := os.Stat(profileDir); err != nil {
					t.Fatalf("browser profile before cleanup: %v", err)
				}
				if cancelRequest {
					cancel()
					if _, err := runner.Render(ctx, Document{}); !errors.Is(err, context.Canceled) {
						t.Fatalf("render cancelled request: got %v, want context.Canceled", err)
					}
				}
			})
			if process == nil {
				if t.Failed() {
					return
				}
				t.Skip("managed Chromium is not prepared")
			}
			if !errors.Is(browserCtx.Err(), context.Canceled) {
				t.Error("test cleanup left the browser context active")
			}
			// The allocator must have waited for its process before cleanup returns.
			// Windows releases the process handle on Wait; Unix marks it done.
			want := os.ErrProcessDone
			if runtime.GOOS == "windows" {
				want = syscall.EINVAL
			}
			if err := process.Signal(syscall.Signal(0)); !errors.Is(err, want) {
				t.Errorf("browser process %d was not reaped: got %v, want %v", process.Pid, err, want)
			}
			if _, err := os.Stat(profileDir); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("browser profile remains after cleanup: %v", err)
			}
			if _, err := runner.Render(context.Background(), Document{}); !errors.Is(err, context.Canceled) {
				t.Fatalf("closed runner restarted a browser: %v", err)
			}
		})
	}
}

func TestChromiumRunnerLeavesOperatorProfileOwnedByCaller(t *testing.T) {
	runner := newTestChromiumRunner(t)
	profile := t.TempDir()
	runner.browserArgs = append(runner.browserArgs, "--user-data-dir="+profile)
	if _, err := runner.Render(t.Context(), Document{Width: 32, Height: 32, Output: "png", HTML: "<!doctype html><html><body>owned</body></html>"}); err != nil {
		t.Fatal(err)
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(profile); err != nil || !info.IsDir() {
		t.Fatalf("operator profile removed: %v", err)
	}
}
