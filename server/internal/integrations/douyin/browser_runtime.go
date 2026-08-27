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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	BrowserModeAuto      = "auto"
	BrowserModeVisible   = "visible"
	BrowserModeHeadless  = "headless"
	BrowserModeRemoteCDP = "remote_cdp"

	douyinCDPDiscoveryTimeout      = 5 * time.Second
	douyinCDPDiscoveryMaxBytes     = 64 << 10
	douyinBrowserDevToolsWait      = 15 * time.Second
	douyinBrowserDevToolsPollEvery = 300 * time.Millisecond
)

type browserLaunchAttempt struct {
	mode               string
	browserPath        string
	remoteDebuggingURL string
	// useProfile 标记该尝试使用持久化浏览器 profile：扫码登录的可见窗口
	// 与登录态搜索（headless）共享同一 profile 目录，两者通过 profileSlot
	// 互斥；未配置 UserDataDir 时该标记无效果。
	useProfile bool
}

func newDouyinBrowserContext(requestCtx context.Context, attempt browserLaunchAttempt, browserArgs []string, userDataDir string) (context.Context, context.CancelFunc, error) {
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
	// 本地启动必须挂载 user-data-dir：持久化 profile 保留设备指纹与
	// sec_sdk 环境（扫码登录可见窗口与登录态搜索共享，降低新设备风控）；
	// 无持久 profile 的尝试使用一次性临时目录，避免浏览器落入系统默认
	// profile（委托既有 Chrome 实例、会话互相污染）。
	effectiveUserDataDir := ""
	tempUserDataDir := ""
	if attempt.useProfile && strings.TrimSpace(userDataDir) != "" {
		effectiveUserDataDir = strings.TrimSpace(userDataDir)
		// chromedp 的 context cancel 在 Windows 上无法保证杀掉完整进程树，
		// 残留进程会占用 profile 导致启动报"无法在现有的会话中打开"。
		killDouyinProfileProcesses(effectiveUserDataDir)
		if err := os.MkdirAll(effectiveUserDataDir, 0o755); err != nil {
			return nil, nil, fmt.Errorf("%w: login browser profile directory is unavailable", thirdparty.ErrQRLoginBrowserUnavailable)
		}
	} else {
		dir, err := os.MkdirTemp("", "rayleabot-douyin-browser-")
		if err != nil {
			return nil, nil, fmt.Errorf("%w: temporary browser profile directory is unavailable", thirdparty.ErrQRLoginBrowserUnavailable)
		}
		tempUserDataDir = dir
		effectiveUserDataDir = dir
	}

	port, err := reserveDouyinBrowserPort()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: browser debugging port is unavailable", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	// chromedp 的 ExecAllocator 通过读取浏览器 stderr 管道发现 DevTools 端口，
	// 但 Edge/Chrome 经 Go 管道启动时不向管道输出 "DevTools listening on" 行
	// （stderr 重定向到文件则正常），chromedp 读不到端口便报启动失败，且其
	// cancel 杀不掉进程树，每次尝试都会泄漏一整套浏览器进程。改为自管启动：
	// 固定端口 + 轮询 /json/version 发现端点，stderr 落到临时文件（文件句柄
	// 语义），进程树由 taskkill 显式回收。
	// 浏览器 stderr 落到固定路径并保留（诊断需要）：每次启动覆盖，
	// 失败后从服务器日志里的 browser_log 路径读取浏览器输出。
	logPath := filepath.Join(os.TempDir(), "rayleabot-douyin-browser.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: browser log file is unavailable", thirdparty.ErrQRLoginBrowserUnavailable)
	}

	command := exec.Command(path, douyinBrowserLaunchArgs(attempt, browserArgs, effectiveUserDataDir, port)...)
	command.Stderr = logFile
	command.Stdout = logFile
	if err := command.Start(); err != nil {
		logFile.Close()
		if tempUserDataDir != "" {
			_ = os.RemoveAll(tempUserDataDir)
		}
		return nil, nil, fmt.Errorf("%w: browser process could not start", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	pid := command.Process.Pid
	go func() { _ = command.Wait() }()

	wsURL, err := waitDouyinBrowserDevTools(requestCtx, port)
	if err != nil {
		if attempt.useProfile {
			// Edge 可能以 relaunch 方式启动：首个进程拉起真正的浏览器
			// 进程后立即退出，记录的 pid 到回收时已不存在。按 profile
			// 路径匹配整树回收最可靠。
			killDouyinProfileProcesses(effectiveUserDataDir)
		} else {
			killDouyinBrowserTree(pid)
		}
		if tempUserDataDir != "" {
			_ = os.RemoveAll(tempUserDataDir)
		}
		logFile.Close()
		return nil, nil, fmt.Errorf("%w: browser debugging endpoint did not become ready", thirdparty.ErrQRLoginBrowserUnavailable)
	}

	allocatorCtx, cancelAllocator := chromedp.NewRemoteAllocator(context.Background(), wsURL)
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	tabCtx, cancelTab := chromedp.NewContext(browserCtx)
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			cancelTab()
			cancelBrowser()
			cancelAllocator()
			if attempt.useProfile {
				killDouyinProfileProcesses(effectiveUserDataDir)
			} else {
				killDouyinBrowserTree(pid)
			}
			if tempUserDataDir != "" {
				_ = os.RemoveAll(tempUserDataDir)
			}
			logFile.Close()
		})
	}
	return tabCtx, cancel, nil
}

// overrideDouyinBrowserUserAgent 把 UA 替换为与实际浏览器版本一致的字符串。
// 硬编码 UA 会与真实 Chromium 版本（navigator.userAgentData 等信号）不一致，
// 被 sec_sdk 视为可疑环境；这里用 CDP 版本信息动态构造。
func overrideDouyinBrowserUserAgent(ctx context.Context) error {
	var product string
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, versionProduct, _, _, _, err := browser.GetVersion().Do(ctx)
		if err != nil {
			return err
		}
		product = strings.TrimSpace(versionProduct)
		return nil
	}))
	if err != nil || product == "" {
		if err == nil {
			err = fmt.Errorf("douyin browser version product is empty")
		}
		return err
	}
	// headless 模式下 GetVersion 返回 HeadlessChrome/…，保留该前缀会向
	// sec_sdk 暴露无头环境；清洗成普通 Chrome 形态与实际渲染引擎一致。
	product = strings.ReplaceAll(product, "HeadlessChrome", "Chrome")
	userAgent := douyinUserAgentForProduct(runtime.GOOS, product)
	return chromedp.Run(ctx, emulation.SetUserAgentOverride(userAgent).WithAcceptLanguage("zh-CN,zh;q=0.9,en;q=0.8"))
}

// douyinUserAgentForProduct 按平台生成与浏览器 product（如 "Chrome/152.0.7977.42"）
// 一致的 UA 字符串。
func douyinUserAgentForProduct(goos, product string) string {
	product = strings.TrimSpace(product)
	if product == "" {
		return ""
	}
	switch goos {
	case "darwin":
		return "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) " + product + " Safari/537.36"
	case "linux", "freebsd", "openbsd":
		return "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) " + product + " Safari/537.36"
	default:
		return "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) " + product + " Safari/537.36"
	}
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
			attempts = append(attempts, browserLaunchAttempt{mode: BrowserModeVisible, browserPath: browserPath, useProfile: true})
		}
		attempts = append(attempts, browserLaunchAttempt{mode: BrowserModeHeadless, browserPath: browserPath})
		return attempts, nil
	case BrowserModeVisible:
		return []browserLaunchAttempt{{mode: BrowserModeVisible, browserPath: browserPath, useProfile: true}}, nil
	case BrowserModeHeadless:
		return []browserLaunchAttempt{{mode: BrowserModeHeadless, browserPath: browserPath}}, nil
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
	// 托管 Chromium 优先于系统浏览器：系统 Edge/Chrome 可能不向 stderr 输出
	// DevTools 端点（chromedp 依赖该输出发现调试端口），且版本不受控，
	// 与 UA 对齐策略冲突；仅当托管浏览器不可用时才回退系统探测。
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

// clearDouyinLoginStateCookies 删除持久化 profile 中残留的登录态 Cookie。
// 保留 ttwid、s_v_web_id、webid、odin_tt 等设备/安全字段，维持设备信誉；
// 只清理会把登录页直接带入"已登录"状态的会话字段。
func clearDouyinLoginStateCookies(ctx context.Context) error {
	names := []string{
		"sessionid", "sessionid_ss", "sid_guard", "sid_tt", "uid_tt", "uid_tt_ss",
		"sid_ucp_v1", "ssid_ucp_v1", "passport_csrf_token", "passport_csrf_token_default",
		"passport_auth_mix_state", "passport_assist_user", "passport_mfa_token",
		"d_ticket", "LOGIN_STATUS", "__ac_nonce",
	}
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		for _, name := range names {
			if err := network.DeleteCookies(name).WithDomain(".douyin.com").Do(ctx); err != nil {
				return err
			}
		}
		return nil
	}))
}

// killDouyinProfileProcesses 结束占用指定 profile 目录的残留 Chromium 进程。
// 仅 Windows 需要：chromedp 通过 context cancel 关闭浏览器时可能留下子进程，
// 导致后续以同一 user-data-dir 启动时报"无法在现有的会话中打开"。
// 需同时匹配 chrome.exe 与 msedge.exe：系统浏览器探测可能回退到 Edge，
// Edge 主进程被杀后子进程树同样会残留。
func killDouyinProfileProcesses(userDataDir string) {
	if runtime.GOOS != "windows" || strings.TrimSpace(userDataDir) == "" {
		return
	}
	// PowerShell -like 是通配符匹配，路径中的 [ ] * ? ` 必须转义，
	// 否则包含特殊字符的 profile 路径匹配不到（或误匹配其他进程）。
	escaped := strings.NewReplacer(
		"'", "''",
		"`", "``",
		"[", "`[",
		"]", "`]",
		"*", "`*",
		"?", "`?",
	).Replace(userDataDir)
	script := "Get-CimInstance Win32_Process | " +
		"Where-Object { $_.Name -in @('chrome.exe','msedge.exe') -and $_.CommandLine -like '*" + escaped + "*' } | " +
		"ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	_ = command.Run()
}

// douyinBrowserLaunchArgs 构建自管启动的浏览器命令行参数。保持与原先
// chromedp ExecAllocator 相同的旗标语义：先加 no-first-run 等基础旗标，
// 挂载 user-data-dir（持久 profile 或调用方准备的一次性临时目录，必挂），
// 再叠加用户配置的 browserArgs（过滤关键旗标），
// 最后是标准反检测旗标与固定调试端口。
func douyinBrowserLaunchArgs(attempt browserLaunchAttempt, configured []string, userDataDir string, port int) []string {
	args := make([]string, 0, len(configured)+40)
	args = append(args, "--no-first-run", "--no-default-browser-check")
	if strings.TrimSpace(userDataDir) != "" {
		args = append(args, "--user-data-dir="+userDataDir)
	}
	args = append(args, douyinConfiguredBrowserArgs(configured)...)
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
	if attempt.mode == BrowserModeHeadless {
		args = append(args, "--headless=new")
	}
	args = append(args, fmt.Sprintf("--remote-debugging-port=%d", port), "about:blank")
	return args
}

// douyinConfiguredBrowserArgs 把用户配置的 browserArgs 转成命令行旗标，
// 过滤掉会破坏启动契约的 headless / user-data-dir / remote-debugging-*。
func douyinConfiguredBrowserArgs(arguments []string) []string {
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

func reserveDouyinBrowserPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	return port, nil
}

// waitDouyinBrowserDevTools 轮询浏览器调试端点直到 /json/version 可访问。
// Edge/Chrome 新 headless 模式经 Go 管道启动时不写 "DevTools listening on"
// 到管道，chromedp 的 stderr 端口发现不可用；固定端口后直接轮询 HTTP 端点。
func waitDouyinBrowserDevTools(ctx context.Context, port int) (string, error) {
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(douyinBrowserDevToolsWait)
	for {
		wsURL, err := resolveDouyinRemoteDebuggingURL(ctx, endpoint, nil)
		if err == nil {
			return wsURL, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("douyin browser debugging endpoint did not become ready")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(douyinBrowserDevToolsPollEvery):
		}
	}
}

// killDouyinBrowserTree 结束浏览器进程树。Windows 上 Kill 只杀主进程，
// 子进程残留会占用 profile；用 taskkill /T 整树回收。
func killDouyinBrowserTree(pid int) {
	if pid <= 0 {
		return
	}
	if runtime.GOOS == "windows" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, "taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
		_ = command.Run()
		return
	}
	process, err := os.FindProcess(pid)
	if err == nil {
		_ = process.Kill()
	}
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
