package render

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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

func newTestChromiumRunner(t *testing.T, browserArgs ...string) *chromiumRunner {
	t.Helper()
	repoRoot := filepath.Join("..", "..", "..")
	browserPath := strings.TrimSpace(os.Getenv("RAYLEA_TEST_BROWSER_PATH"))
	if browserPath == "" {
		var err error
		browserPath, err = deps.NewManager(repoRoot).ResolvePreparedEntrypoint("chromium", "browser")
		if err != nil {
			t.Skipf("managed chromium is not prepared: %v", err)
		}
	}

	logTestBrowserVersion(t, browserPath)
	output := &testBrowserOutput{}
	runner := NewChromiumRunner(ChromiumOptions{BrowserPath: browserPath, BrowserArgs: browserArgs, CombinedOutput: output, TempRoot: t.TempDir()})
	t.Cleanup(func() {
		if err := runner.Close(); err != nil {
			t.Errorf("close test Chromium runner: %v", err)
		}
		if t.Failed() {
			t.Logf("Chromium combined output after Close (last %d bytes):\n%s", testBrowserOutputLimit, output.String())
		}
	})
	return runner
}

// newTestAssetServer serves handler to pages in the test browser and returns the
// browser argument that allows its port. Browsers block the Fetch standard's bad
// ports, which systems such as Windows with a low dynamic port range can assign
// as ephemeral ports; page requests would then fail inside the browser instead of
// reaching handler.
func newTestAssetServer(t *testing.T, handler http.Handler) (*httptest.Server, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("parse test asset server address: %v", err)
	}
	return server, "--explicitly-allowed-ports=" + port
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
	version, err := testBrowserVersion(ctx, browserPath)
	t.Logf("selected Chromium version: %s (probe error: %v)", version, err)
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
				if output := runner.combinedOutput.(*testBrowserOutput).String(); !strings.Contains(output, "DevTools listening on") {
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

func TestChromiumRunnerUsesCacheWithUnavailableSystemTemp(t *testing.T) {
	runner := newTestChromiumRunner(t)
	blocked := filepath.Join(t.TempDir(), "blocked-system-temp")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(key, blocked)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	content, err := runner.Render(ctx, Document{Width: 48, Height: 32, Output: "png", HTML: "<!doctype html><html><body style='margin:0;background:#267'>cache</body></html>"})
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(content))
	if err != nil || img.Bounds().Dx() != 48 || img.Bounds().Dy() != 32 {
		t.Fatalf("invalid rendered PNG: %v", err)
	}
	profile := runner.profileDir
	if filepath.Dir(profile) != runner.tempRoot {
		t.Fatalf("browser profile escaped cache: %q", profile)
	}
	wantTempRoot := runner.tempRoot
	if runtime.GOOS != "windows" {
		wantTempRoot = "/tmp"
	}
	for _, key := range []string{"TMP", "TEMP", "TMPDIR"} {
		value := ""
		for _, item := range runner.command.Env {
			name, candidate, _ := strings.Cut(item, "=")
			if strings.EqualFold(name, key) {
				value = candidate
			}
		}
		if value != wantTempRoot {
			t.Fatalf("browser %s = %q, want %q", key, value, wantTempRoot)
		}
	}
	entries, err := os.ReadDir(runner.tempRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "rayleabot-render-") {
			t.Fatalf("render workspace survived successful capture: %s", entry.Name())
		}
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatalf("browser profile survived cleanup: %v", err)
	}
}
