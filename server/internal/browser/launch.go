package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	cdpDiscoveryTimeout      = 5 * time.Second
	cdpDiscoveryMaxBytes     = 64 << 10
	browserDevToolsWait      = 15 * time.Second
	browserDevToolsPollEvery = 300 * time.Millisecond
	browserShutdownTimeout   = 10 * time.Second
)

// launchLocalBrowser starts one local Chromium-family process with a
// reserved debugging port, waits for its CDP endpoint, and returns the
// browser-level WebSocket URL plus a cleanup function that reaps the whole
// process tree.
func launchLocalBrowser(ctx context.Context, options Options, pluginID, profile string, attempt launchAttempt) (string, func(), error) {
	path := strings.TrimSpace(attempt.browserPath)
	if path == "" {
		return "", nil, fmt.Errorf("%w: Chromium executable is missing", ErrUnavailable)
	}
	userDataDir := ""
	tempUserDataDir := ""
	if attempt.useProfile {
		userDataDir = filepath.Join(options.ProfileRoot, pluginID, profile)
		if err := os.MkdirAll(userDataDir, 0o755); err != nil {
			return "", nil, fmt.Errorf("%w: browser profile directory is unavailable", ErrUnavailable)
		}
	} else {
		dir, err := os.MkdirTemp("", "rayleabot-browser-")
		if err != nil {
			return "", nil, fmt.Errorf("%w: temporary browser profile directory is unavailable", ErrUnavailable)
		}
		tempUserDataDir = dir
		userDataDir = dir
	}

	port, err := reserveBrowserPort()
	if err != nil {
		if tempUserDataDir != "" {
			_ = os.RemoveAll(tempUserDataDir)
		}
		return "", nil, fmt.Errorf("%w: browser debugging port is unavailable", ErrUnavailable)
	}
	logFile, err := os.CreateTemp("", "rayleabot-plugin-browser-*.log")
	if err != nil {
		if tempUserDataDir != "" {
			_ = os.RemoveAll(tempUserDataDir)
		}
		return "", nil, fmt.Errorf("%w: browser log file is unavailable", ErrUnavailable)
	}

	command := exec.Command(path, browserLaunchArgs(attempt, options.BrowserArgs, userDataDir, port)...)
	command.Stderr = logFile
	command.Stdout = logFile
	terminate, err := startBrowserProcess(command)
	if err != nil {
		_ = logFile.Close()
		_ = os.Remove(logFile.Name())
		if tempUserDataDir != "" {
			_ = os.RemoveAll(tempUserDataDir)
		}
		return "", nil, fmt.Errorf("%w: browser process could not start", ErrUnavailable)
	}
	stopped := make(chan struct{})
	go func() { _ = command.Wait(); close(stopped) }()
	stop := func() {
		terminate()
		select {
		case <-stopped:
		case <-time.After(browserShutdownTimeout):
		}
	}

	wsURL, err := waitBrowserDevTools(ctx, port)
	if err != nil {
		stop()
		if tempUserDataDir != "" {
			_ = os.RemoveAll(tempUserDataDir)
		}
		_ = logFile.Close()
		_ = os.Remove(logFile.Name())
		return "", nil, fmt.Errorf("%w: browser debugging endpoint did not become ready", ErrUnavailable)
	}

	cleanup := func() {
		stop()
		if tempUserDataDir != "" {
			_ = os.RemoveAll(tempUserDataDir)
		}
		_ = logFile.Close()
		_ = os.Remove(logFile.Name())
	}
	return wsURL, cleanup, nil
}

func resolveBrowserPath(configuredPath, managedPath string) string {
	return resolveBrowserPathWith(
		configuredPath,
		managedPath,
		func() string { return findSystemChromium(runtime.GOOS, os.Getenv, exec.LookPath, fileExists) },
		fileExists,
	)
}

func resolveBrowserPathWith(configuredPath, managedPath string, findSystem func() string, exists func(string) bool) string {
	if path := strings.TrimSpace(configuredPath); path != "" && exists(path) {
		return path
	}
	if path := strings.TrimSpace(managedPath); path != "" && exists(path) {
		return path
	}
	return strings.TrimSpace(findSystem())
}

func findSystemChromium(goos string, getenv func(string) string, lookPath func(string) (string, error), exists func(string) bool) string {
	var candidates []string
	switch goos {
	case "windows":
		programFiles := strings.TrimSpace(getenv("ProgramFiles"))
		programFilesX86 := strings.TrimSpace(getenv("ProgramFiles(x86)"))
		localAppData := strings.TrimSpace(getenv("LOCALAPPDATA"))
		for _, candidate := range []struct {
			root  string
			parts []string
		}{
			{root: programFiles, parts: []string{"Google", "Chrome", "Application", "chrome.exe"}},
			{root: programFilesX86, parts: []string{"Google", "Chrome", "Application", "chrome.exe"}},
			{root: localAppData, parts: []string{"Google", "Chrome", "Application", "chrome.exe"}},
			{root: programFiles, parts: []string{"Microsoft", "Edge", "Application", "msedge.exe"}},
			{root: programFilesX86, parts: []string{"Microsoft", "Edge", "Application", "msedge.exe"}},
		} {
			if candidate.root != "" {
				candidates = append(candidates, filepath.Join(append([]string{candidate.root}, candidate.parts...)...))
			}
		}
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) != "" && exists(candidate) {
			return candidate
		}
	}
	for _, name := range []string{"google-chrome-stable", "google-chrome", "chromium", "chromium-browser", "microsoft-edge", "msedge"} {
		if path, err := lookPath(name); err == nil && strings.TrimSpace(path) != "" {
			return path
		}
	}
	return ""
}

func hasInteractiveDesktop() bool {
	switch runtime.GOOS {
	case "windows", "darwin":
		return true
	case "linux", "freebsd", "openbsd":
		return strings.TrimSpace(os.Getenv("DISPLAY")) != "" || strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
	default:
		return false
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// browserLaunchArgs builds a hardened local launch command line. Configured
// arguments cannot override the profile, headless, or remote-debugging
// contract flags the host owns.
func browserLaunchArgs(attempt launchAttempt, configured []string, userDataDir string, port int) []string {
	args := make([]string, 0, len(configured)+40)
	args = append(args, "--no-first-run", "--no-default-browser-check")
	if strings.TrimSpace(userDataDir) != "" {
		args = append(args, "--user-data-dir="+userDataDir)
	}
	args = append(args, configuredBrowserArgs(configured)...)
	args = append(args,
		"--accept-lang=zh-CN,zh;q=0.9,en;q=0.8",
		"--lang=zh-CN",
		"--disable-blink-features=AutomationControlled",
		"--disable-features=IsolateOrigins,site-per-process,TranslateUI,BlinkRuntimeCallStats,OptimizationHints,MediaRouter",
		"--no-sandbox",
		"--disable-setuid-sandbox",
		"--disable-dev-shm-usage",
		"--disable-infobars",
		"--mute-audio",
		"--disable-sync",
		"--metrics-recording-only",
		"--disable-background-networking",
		"--disable-backgrounding-occluded-windows",
		"--disable-renderer-backgrounding",
		"--disable-breakpad",
		"--disable-client-side-phishing-detection",
		"--disable-default-apps",
		"--disable-extensions",
		"--disable-hang-monitor",
		"--disable-ipc-flooding-protection",
		"--disable-popup-blocking",
		"--disable-prompt-on-repost",
		"--safebrowsing-disable-auto-update",
		"--password-store=basic",
		"--window-size=1920,1080",
		"--force-color-profile=srgb",
		"--force-fieldtrials=WebRTC-MultipleRoutes/Disabled/",
	)
	if attempt.mode == ModeHeadless {
		args = append(args, "--headless=new")
	}
	args = append(args, fmt.Sprintf("--remote-debugging-port=%d", port), "about:blank")
	return args
}

func configuredBrowserArgs(arguments []string) []string {
	flags := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		argument = strings.TrimSpace(strings.TrimPrefix(argument, "--"))
		if argument == "" {
			continue
		}
		key, value, hasValue := strings.Cut(argument, "=")
		key = strings.TrimSpace(strings.ToLower(key))
		switch key {
		case "headless", "user-data-dir", "remote-debugging-address", "remote-debugging-port", "remote-debugging-pipe":
			continue
		}
		if hasValue {
			flags = append(flags, "--"+key+"="+value)
			continue
		}
		flags = append(flags, "--"+key)
	}
	return flags
}

func reserveBrowserPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port, nil
}

func waitBrowserDevTools(ctx context.Context, port int) (string, error) {
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(browserDevToolsWait)
	for {
		wsURL, err := resolveRemoteDebuggingURL(ctx, endpoint)
		if err == nil {
			return wsURL, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("browser debugging endpoint did not become ready")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(browserDevToolsPollEvery):
		}
	}
}

func resolveRemoteDebuggingURL(ctx context.Context, raw string) (string, error) {
	if err := validateRemoteDebuggingURL(raw); err != nil {
		return "", err
	}
	endpoint, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("%w: remote CDP endpoint is invalid", ErrInvalidRequest)
	}
	endpoint.Scheme = strings.ToLower(endpoint.Scheme)
	if isDirectWebSocketEndpoint(endpoint) {
		return endpoint.String(), nil
	}

	discoveryScheme := "http"
	requireSecureWebSocket := false
	switch endpoint.Scheme {
	case "https", "wss":
		discoveryScheme = "https"
		requireSecureWebSocket = true
	case "http", "ws":
	default:
		return "", fmt.Errorf("%w: remote CDP endpoint uses an unsupported scheme", ErrInvalidRequest)
	}
	discoveryURL := *endpoint
	discoveryURL.Scheme = discoveryScheme
	discoveryURL.Path = "/json/version"
	discoveryURL.RawPath = ""
	discoveryURL.RawQuery = ""
	discoveryURL.ForceQuery = false

	discoveryClient := http.Client{Timeout: cdpDiscoveryTimeout}
	discoveryClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("%w: remote CDP discovery request is invalid", ErrInvalidRequest)
	}
	response, err := discoveryClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: remote CDP discovery failed", ErrUnavailable)
	}
	defer func(release func() error) { _ = release() }(response.Body.Close)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: remote CDP discovery returned HTTP %d", ErrUnavailable, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, cdpDiscoveryMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("%w: remote CDP discovery response could not be read", ErrUnavailable)
	}
	if len(body) > cdpDiscoveryMaxBytes {
		return "", fmt.Errorf("%w: remote CDP discovery response is too large", ErrUnavailable)
	}
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &version); err != nil || strings.TrimSpace(version.WebSocketDebuggerURL) == "" {
		return "", fmt.Errorf("%w: remote CDP discovery response is invalid", ErrUnavailable)
	}
	if err := validateRemoteDebuggingURL(version.WebSocketDebuggerURL); err != nil {
		return "", err
	}
	webSocketEndpoint, err := url.Parse(strings.TrimSpace(version.WebSocketDebuggerURL))
	if err != nil || !isDirectWebSocketEndpoint(webSocketEndpoint) {
		return "", fmt.Errorf("%w: remote CDP discovery did not return a browser WebSocket endpoint", ErrUnavailable)
	}
	if requireSecureWebSocket && !strings.EqualFold(webSocketEndpoint.Scheme, "wss") {
		return "", fmt.Errorf("%w: secure remote CDP discovery returned an insecure WebSocket endpoint", ErrUnavailable)
	}
	return webSocketEndpoint.String(), nil
}

func isDirectWebSocketEndpoint(endpoint *url.URL) bool {
	if endpoint == nil {
		return false
	}
	scheme := strings.ToLower(endpoint.Scheme)
	if scheme != "ws" && scheme != "wss" {
		return false
	}
	const prefix = "/devtools/browser/"
	return strings.HasPrefix(endpoint.EscapedPath(), prefix) && strings.TrimPrefix(endpoint.EscapedPath(), prefix) != ""
}

func validateRemoteDebuggingURL(raw string) error {
	endpoint, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return fmt.Errorf("%w: remote CDP endpoint must be a loopback HTTP(S) or WS(S) address without credentials", ErrInvalidRequest)
	}
	switch strings.ToLower(endpoint.Scheme) {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf("%w: remote CDP endpoint uses an unsupported scheme", ErrInvalidRequest)
	}
	host := strings.Trim(strings.TrimSpace(endpoint.Hostname()), "[]")
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("%w: remote CDP endpoint must use a loopback host", ErrInvalidRequest)
		}
	}
	return nil
}
