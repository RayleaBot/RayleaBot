package render

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

	logTestBrowserVersion(t, browserPath)
	output := &testBrowserOutput{}
	trace := newBrowserDiagnosticTrace()
	runner := NewChromiumRunner(ChromiumOptions{
		BrowserPath:    browserPath,
		CombinedOutput: &browserDiagnosticOutput{output: output, trace: trace},
		Debugf:         trace.debugf,
	})
	t.Cleanup(func() {
		if err := runner.Close(); err != nil {
			t.Errorf("close test Chromium runner: %v", err)
		}
		if t.Failed() {
			t.Logf("Chromium combined output after Close (last %d bytes):\n%s", testBrowserOutputLimit, output.String())
		}
		t.Logf("Browser diagnostic timeline (payload omitted):\n%s", trace.snapshot())
	})
	return runner
}

func logTestBrowserVersion(t *testing.T, browserPath string) {
	t.Helper()
	absolutePath, err := filepath.Abs(browserPath)
	if err != nil {
		t.Logf("selected Chromium executable: %s (absolute path error: %v)", browserPath, err)
	} else {
		t.Logf("selected Chromium executable: %s", absolutePath)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, browserPath, "--version")
	if runtime.GOOS == "windows" {
		// Windows browsers may treat --version as a normal launch. Read PE
		// metadata instead, without opening the operator's browser profile.
		script := "$ErrorActionPreference='Stop'; [Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false); " +
			"(Get-Item -LiteralPath '" + strings.ReplaceAll(browserPath, "'", "''") + "').VersionInfo | " +
			"Select-Object ProductName,FileVersion,ProductVersion | ConvertTo-Json -Compress"
		command = exec.CommandContext(ctx, filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"),
			"-NoLogo", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	}
	prepareBrowserCommand(command)
	command.WaitDelay = time.Second
	output := &testBrowserOutput{}
	command.Stdout, command.Stderr = output, output
	err = command.Run()
	t.Logf("selected Chromium version: %s (probe error: %v)", strings.TrimSpace(output.String()), err)
}

const testBrowserOutputLimit = 32 * 1024

// The browser writes asynchronously, including while failed startup is being
// cancelled. Retain only its latest output without blocking cleanup on a pipe.
type testBrowserOutput struct {
	mu   sync.Mutex
	data []byte
}

func (b *testBrowserOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	if n >= testBrowserOutputLimit {
		b.data = append(b.data[:0], p[n-testBrowserOutputLimit:]...)
	} else {
		if overflow := len(b.data) + n - testBrowserOutputLimit; overflow > 0 {
			b.data = b.data[:copy(b.data, b.data[overflow:])]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}

func (b *testBrowserOutput) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.data)
}

func TestBrowserOutputRetainsBoundedTail(t *testing.T) {
	output := &testBrowserOutput{}
	initial := strings.Repeat("a", testBrowserOutputLimit+8)
	if n, err := output.Write([]byte(initial)); n != len(initial) || err != nil {
		t.Fatalf("write oversized output: %d, %v", n, err)
	}
	if _, err := output.Write([]byte("last")); err != nil {
		t.Fatal(err)
	}
	if got := output.String(); got != strings.Repeat("a", testBrowserOutputLimit-4)+"last" {
		t.Fatalf("output tail mismatch: length=%d", len(got))
	}
	var writers sync.WaitGroup
	for range 8 {
		writers.Go(func() {
			for range 100 {
				_, _ = output.Write([]byte("concurrent browser output\n"))
				_ = output.String()
			}
		})
	}
	writers.Wait()
	if got := len(output.String()); got != testBrowserOutputLimit {
		t.Fatalf("concurrent output exceeded limit: %d", got)
	}
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
				if output := runner.combinedOutput.(*browserDiagnosticOutput).output.String(); !strings.Contains(output, "DevTools listening on") {
					t.Fatal("browser startup output was not captured")
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
