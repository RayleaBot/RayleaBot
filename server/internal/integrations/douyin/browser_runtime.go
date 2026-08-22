package douyin

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

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	BrowserModeAuto      = "auto"
	BrowserModeVisible   = "visible"
	BrowserModeHeadless  = "headless"
	BrowserModeRemoteCDP = "remote_cdp"

	douyinCDPDiscoveryTimeout  = 5 * time.Second
	douyinCDPDiscoveryMaxBytes = 64 << 10
)

type browserLaunchAttempt struct {
	mode               string
	browserPath        string
	remoteDebuggingURL string
}

func newDouyinBrowserContext(requestCtx context.Context, attempt browserLaunchAttempt, browserArgs []string) (context.Context, context.CancelFunc, error) {
	if attempt.mode == BrowserModeRemoteCDP {
		if strings.TrimSpace(attempt.remoteDebuggingURL) == "" {
			return nil, nil, fmt.Errorf("%w: remote CDP endpoint is missing", thirdparty.ErrQRLoginBrowserUnavailable)
		}
		webSocketURL, err := resolveDouyinRemoteDebuggingURL(requestCtx, attempt.remoteDebuggingURL, nil)
		if err != nil {
			return nil, nil, err
		}
		allocatorCtx, cancelAllocator := chromedp.NewRemoteAllocator(context.Background(), webSocketURL)
		browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
		tabCtx, cancelTab := chromedp.NewContext(browserCtx)
		return tabCtx, func() {
			cancelTab()
			cancelBrowser()
			cancelAllocator()
		}, nil
	}

	path := strings.TrimSpace(attempt.browserPath)
	if path == "" {
		return nil, nil, fmt.Errorf("%w: Chromium executable is missing", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	allocatorOptions := []chromedp.ExecAllocatorOption{
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,
		chromedp.ExecPath(path),
	}
	allocatorOptions = append(allocatorOptions, douyinAllocatorFlags(browserArgs)...)
	allocatorOptions = append(allocatorOptions,
		chromedp.Flag("user-agent", douyinUserAgent),
		chromedp.Flag("accept-lang", "zh-CN,zh;q=0.9,en;q=0.8"),
		chromedp.Flag("lang", "zh-CN"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-features", "IsolateOrigins,site-per-process,TranslateUI,BlinkRuntimeCallStats,OptimizationHints,MediaRouter"),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("mute-audio", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("force-fieldtrials", "WebRTC-MultipleRoutes/Disabled/"),
	)
	if attempt.mode == BrowserModeHeadless {
		allocatorOptions = append(allocatorOptions, chromedp.Flag("headless", "new"))
	}

	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), allocatorOptions...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	tabCtx, cancelTab := chromedp.NewContext(browserCtx)
	return tabCtx, func() {
		cancelTab()
		cancelBrowser()
		cancelAllocator()
	}, nil
}

func resolveDouyinRemoteDebuggingURL(ctx context.Context, raw string, client *http.Client) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateDouyinRemoteDebuggingURL(raw); err != nil {
		return "", err
	}
	endpoint, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("%w: remote CDP endpoint is invalid", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	endpoint.Scheme = strings.ToLower(endpoint.Scheme)
	if isDouyinDirectWebSocketEndpoint(endpoint) {
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
		return "", fmt.Errorf("%w: remote CDP endpoint uses an unsupported scheme", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	discoveryURL := *endpoint
	discoveryURL.Scheme = discoveryScheme
	discoveryURL.Path = "/json/version"
	discoveryURL.RawPath = ""
	discoveryURL.RawQuery = ""
	discoveryURL.ForceQuery = false

	discoveryClient := http.Client{Timeout: douyinCDPDiscoveryTimeout}
	if client != nil {
		discoveryClient = *client
		if discoveryClient.Timeout <= 0 || discoveryClient.Timeout > douyinCDPDiscoveryTimeout {
			discoveryClient.Timeout = douyinCDPDiscoveryTimeout
		}
	}
	discoveryClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("%w: remote CDP discovery request is invalid", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	response, err := discoveryClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: remote CDP discovery failed", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("%w: remote CDP discovery returned HTTP %d", thirdparty.ErrQRLoginBrowserUnavailable, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, douyinCDPDiscoveryMaxBytes+1))
	if err != nil {
		return "", fmt.Errorf("%w: remote CDP discovery response could not be read", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	if len(body) > douyinCDPDiscoveryMaxBytes {
		return "", fmt.Errorf("%w: remote CDP discovery response is too large", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &version); err != nil || strings.TrimSpace(version.WebSocketDebuggerURL) == "" {
		return "", fmt.Errorf("%w: remote CDP discovery response is invalid", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	if err := validateDouyinRemoteDebuggingURL(version.WebSocketDebuggerURL); err != nil {
		return "", err
	}
	webSocketEndpoint, err := url.Parse(strings.TrimSpace(version.WebSocketDebuggerURL))
	if err != nil || !isDouyinDirectWebSocketEndpoint(webSocketEndpoint) {
		return "", fmt.Errorf("%w: remote CDP discovery did not return a browser WebSocket endpoint", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	if requireSecureWebSocket && !strings.EqualFold(webSocketEndpoint.Scheme, "wss") {
		return "", fmt.Errorf("%w: secure remote CDP discovery returned an insecure WebSocket endpoint", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	return webSocketEndpoint.String(), nil
}

func isDouyinDirectWebSocketEndpoint(endpoint *url.URL) bool {
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

func douyinBrowserLaunchAttempts(mode, remoteDebuggingURL, configuredPath, managedPath string) ([]browserLaunchAttempt, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	remoteDebuggingURL = strings.TrimSpace(remoteDebuggingURL)
	if remoteDebuggingURL != "" {
		if err := validateDouyinRemoteDebuggingURL(remoteDebuggingURL); err != nil {
			return nil, err
		}
	}
	path := resolveDouyinBrowserPath(configuredPath, managedPath)
	return buildDouyinBrowserLaunchAttempts(mode, remoteDebuggingURL, path, douyinHasInteractiveDesktop(runtime.GOOS, os.Getenv))
}

func buildDouyinBrowserLaunchAttempts(mode, remoteDebuggingURL, browserPath string, interactive bool) ([]browserLaunchAttempt, error) {
	switch mode {
	case BrowserModeAuto:
		attempts := make([]browserLaunchAttempt, 0, 3)
		if remoteDebuggingURL != "" {
			attempts = append(attempts, browserLaunchAttempt{mode: BrowserModeRemoteCDP, remoteDebuggingURL: remoteDebuggingURL})
		}
		if interactive {
			attempts = append(attempts, browserLaunchAttempt{mode: BrowserModeVisible, browserPath: browserPath})
		}
		attempts = append(attempts, browserLaunchAttempt{mode: BrowserModeHeadless, browserPath: browserPath})
		return attempts, nil
	case BrowserModeVisible, BrowserModeHeadless:
		return []browserLaunchAttempt{{mode: mode, browserPath: browserPath}}, nil
	case BrowserModeRemoteCDP:
		if remoteDebuggingURL == "" {
			return nil, fmt.Errorf("%w: remote CDP endpoint is missing", thirdparty.ErrQRLoginBrowserUnavailable)
		}
		return []browserLaunchAttempt{{mode: mode, remoteDebuggingURL: remoteDebuggingURL}}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported browser mode", thirdparty.ErrQRLoginBrowserUnavailable)
	}
}

func resolveDouyinBrowserPath(configuredPath, managedPath string) string {
	return resolveDouyinBrowserPathWith(
		configuredPath,
		managedPath,
		func() string { return findSystemChromium(runtime.GOOS, os.Getenv, exec.LookPath, fileExists) },
		fileExists,
	)
}

func resolveDouyinBrowserPathWith(configuredPath, managedPath string, findSystem func() string, exists func(string) bool) string {
	if path := strings.TrimSpace(configuredPath); path != "" && exists(path) {
		return path
	}
	if path := strings.TrimSpace(findSystem()); path != "" {
		return path
	}
	return strings.TrimSpace(managedPath)
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

func validateDouyinRemoteDebuggingURL(raw string) error {
	endpoint, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.Fragment != "" {
		return fmt.Errorf("%w: remote CDP endpoint must be a loopback HTTP(S) or WS(S) address without credentials", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	switch strings.ToLower(endpoint.Scheme) {
	case "http", "https", "ws", "wss":
	default:
		return fmt.Errorf("%w: remote CDP endpoint uses an unsupported scheme", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	host := strings.Trim(strings.TrimSpace(endpoint.Hostname()), "[]")
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("%w: remote CDP endpoint must use a loopback host", thirdparty.ErrQRLoginBrowserUnavailable)
		}
	}
	return nil
}

func douyinHasInteractiveDesktop(goos string, getenv func(string) string) bool {
	switch goos {
	case "windows", "darwin":
		return true
	case "linux", "freebsd", "openbsd":
		return strings.TrimSpace(getenv("DISPLAY")) != "" || strings.TrimSpace(getenv("WAYLAND_DISPLAY")) != ""
	default:
		return false
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func douyinNetworkCookies(cookies []*network.Cookie) map[string]string {
	values := map[string]string{}
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}
		name := strings.TrimSpace(cookie.Name)
		value := strings.TrimSpace(cookie.Value)
		domain := strings.ToLower(strings.TrimSpace(cookie.Domain))
		if name == "" || value == "" {
			continue
		}
		if thirdparty.HostMatches(domain, "douyin.com", "amemv.com", "bytedance.com") {
			values[name] = value
		}
	}
	return values
}

func douyinAllocatorFlags(arguments []string) []chromedp.ExecAllocatorOption {
	flags := make([]chromedp.ExecAllocatorOption, 0, len(arguments))
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
			flags = append(flags, chromedp.Flag(key, value))
			continue
		}
		flags = append(flags, chromedp.Flag(key, true))
	}
	return flags
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cookieHeaderFromMap(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	keys := make([]string, 0, len(cookies))
	for key, value := range cookies {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sortStrings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+cookies[key])
	}
	return strings.Join(parts, "; ") + ";"
}

func sortStrings(values []string) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			if values[i] > values[j] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
