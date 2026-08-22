package douyin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func TestDouyinNetworkCookiesRequiresDomainBoundary(t *testing.T) {
	values := douyinNetworkCookies([]*network.Cookie{
		{Name: "kept", Value: "1", Domain: ".douyin.com"},
		{Name: "evil", Value: "1", Domain: "douyin.com.evil.test"},
		{Name: "prefix", Value: "1", Domain: "evildouyin.com"},
	})

	if values["kept"] != "1" {
		t.Fatalf("douyinNetworkCookies did not keep valid domain cookie")
	}
	if _, ok := values["evil"]; ok {
		t.Fatalf("douyinNetworkCookies kept suffix-confused domain cookie")
	}
	if _, ok := values["prefix"]; ok {
		t.Fatalf("douyinNetworkCookies kept prefix-confused domain cookie")
	}
}

func TestStartDouyinBrowserContextKeepsInitializationContextAlive(t *testing.T) {
	t.Parallel()

	tabCtx, cancelBrowser := context.WithCancel(context.Background())
	var initializationCtx context.Context
	err := startDouyinBrowserContextWith(
		context.Background(),
		tabCtx,
		cancelBrowser,
		time.Second,
		func(runCtx context.Context) error {
			initializationCtx = runCtx
			return nil
		},
	)
	if err != nil {
		t.Fatalf("startDouyinBrowserContextWith returned error: %v", err)
	}
	if initializationCtx == nil {
		t.Fatal("browser initialization did not receive a context")
	}
	if err := initializationCtx.Err(); err != nil {
		t.Fatalf("browser initialization context was cancelled after the first run: %v", err)
	}

	cancelBrowser()
	if err := initializationCtx.Err(); err != context.Canceled {
		t.Fatalf("browser initialization context error = %v, want context.Canceled", err)
	}
}

func TestDouyinHasInteractiveDesktop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goos string
		env  map[string]string
		want bool
	}{
		{name: "windows", goos: "windows", want: true},
		{name: "macos", goos: "darwin", want: true},
		{name: "linux x11", goos: "linux", env: map[string]string{"DISPLAY": ":0"}, want: true},
		{name: "linux wayland", goos: "linux", env: map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, want: true},
		{name: "linux server", goos: "linux", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			getenv := func(key string) string { return tt.env[key] }
			if got := douyinHasInteractiveDesktop(tt.goos, getenv); got != tt.want {
				t.Fatalf("douyinHasInteractiveDesktop() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindSystemChromiumUsesPathLookupFallback(t *testing.T) {
	t.Parallel()

	got := findSystemChromium(
		"linux",
		func(string) string { return "" },
		func(name string) (string, error) {
			if name == "chromium" {
				return "/fixture/chromium", nil
			}
			return "", fmt.Errorf("not found")
		},
		func(string) bool { return false },
	)
	if got != "/fixture/chromium" {
		t.Fatalf("findSystemChromium() = %q", got)
	}
}

func TestResolveDouyinBrowserPathUsesConfiguredSystemManagedOrder(t *testing.T) {
	t.Parallel()

	exists := func(path string) bool { return path == "/configured/chrome" }
	if got := resolveDouyinBrowserPathWith("/configured/chrome", "/managed/chrome", func() string { return "/system/chrome" }, exists); got != "/configured/chrome" {
		t.Fatalf("configured browser path = %q", got)
	}
	if got := resolveDouyinBrowserPathWith("/missing/chrome", "/managed/chrome", func() string { return "/system/chrome" }, exists); got != "/system/chrome" {
		t.Fatalf("system browser path = %q", got)
	}
	if got := resolveDouyinBrowserPathWith("", "/managed/chrome", func() string { return "" }, exists); got != "/managed/chrome" {
		t.Fatalf("managed browser path = %q", got)
	}
}

func TestFindSystemChromiumSkipsRelativeCandidatesWhenWindowsRootsAreEmpty(t *testing.T) {
	t.Parallel()

	got := findSystemChromium(
		"windows",
		func(string) string { return "" },
		func(name string) (string, error) {
			if name == "msedge" {
				return `C:\fixture\msedge.exe`, nil
			}
			return "", fmt.Errorf("not found")
		},
		func(string) bool { return true },
	)
	if got != `C:\fixture\msedge.exe` {
		t.Fatalf("findSystemChromium() = %q", got)
	}
}

func TestBuildDouyinBrowserLaunchAttempts(t *testing.T) {
	t.Parallel()

	attempts, err := buildDouyinBrowserLaunchAttempts(BrowserModeAuto, "http://127.0.0.1:9222", "/fixture/chrome", true)
	if err != nil {
		t.Fatalf("buildDouyinBrowserLaunchAttempts() error = %v", err)
	}
	if len(attempts) != 3 || attempts[0].mode != BrowserModeRemoteCDP || attempts[1].mode != BrowserModeVisible || attempts[2].mode != BrowserModeHeadless {
		t.Fatalf("auto attempts = %#v", attempts)
	}

	attempts, err = buildDouyinBrowserLaunchAttempts(BrowserModeAuto, "", "/fixture/chrome", false)
	if err != nil {
		t.Fatalf("headless auto attempts error = %v", err)
	}
	if len(attempts) != 1 || attempts[0].mode != BrowserModeHeadless {
		t.Fatalf("server auto attempts = %#v", attempts)
	}

	for _, mode := range []string{BrowserModeVisible, BrowserModeHeadless, BrowserModeRemoteCDP} {
		remoteURL := ""
		if mode == BrowserModeRemoteCDP {
			remoteURL = "ws://[::1]:9222/devtools/browser/fixture"
		}
		attempts, err = buildDouyinBrowserLaunchAttempts(mode, remoteURL, "/fixture/chrome", true)
		if err != nil {
			t.Fatalf("explicit %s attempt error = %v", mode, err)
		}
		if len(attempts) != 1 || attempts[0].mode != mode {
			t.Fatalf("explicit %s attempts = %#v", mode, attempts)
		}
	}
}

func TestValidateDouyinRemoteDebuggingURL(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{
		"http://127.0.0.1:9222",
		"https://localhost",
		"https://localhost:9222/json/version",
		"ws://[::1]:9222/devtools/browser/fixture",
		"wss://localhost/devtools/browser/fixture",
	} {
		if err := validateDouyinRemoteDebuggingURL(endpoint); err != nil {
			t.Fatalf("validateDouyinRemoteDebuggingURL(%q) error = %v", endpoint, err)
		}
	}
	for _, endpoint := range []string{
		"https://example.com:9222",
		"http://user:password@127.0.0.1:9222",
		"file:///tmp/devtools",
	} {
		if err := validateDouyinRemoteDebuggingURL(endpoint); err == nil {
			t.Fatalf("validateDouyinRemoteDebuggingURL(%q) succeeded", endpoint)
		}
	}
}

func TestResolveDouyinRemoteDebuggingURLAcceptsDirectBrowserWebSocket(t *testing.T) {
	t.Parallel()

	const endpoint = "ws://127.0.0.1:9222/devtools/browser/fixture"
	got, err := resolveDouyinRemoteDebuggingURL(context.Background(), endpoint, nil)
	if err != nil {
		t.Fatalf("resolveDouyinRemoteDebuggingURL() error = %v", err)
	}
	if got != endpoint {
		t.Fatalf("resolved endpoint = %q, want %q", got, endpoint)
	}
}

func TestResolveDouyinRemoteDebuggingURLUsesBoundedTypedDiscovery(t *testing.T) {
	t.Parallel()

	requestedPath := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestedPath <- request.URL.Path
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/browser/fixture"}`))
	}))
	defer server.Close()

	got, err := resolveDouyinRemoteDebuggingURL(context.Background(), server.URL+"/ignored/path?secret=fixture", server.Client())
	if err != nil {
		t.Fatalf("resolveDouyinRemoteDebuggingURL() error = %v", err)
	}
	if path := <-requestedPath; path != "/json/version" {
		t.Fatalf("discovery path = %q, want /json/version", path)
	}
	if got != "ws://127.0.0.1:9222/devtools/browser/fixture" {
		t.Fatalf("resolved endpoint = %q", got)
	}
}

func TestResolveDouyinRemoteDebuggingURLSupportsSecureDiscovery(t *testing.T) {
	t.Parallel()

	secureServer := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		webSocketURL := "wss://" + request.Host + "/devtools/browser/fixture"
		_, _ = fmt.Fprintf(response, `{"webSocketDebuggerUrl":%q}`, webSocketURL)
	}))
	defer secureServer.Close()

	got, err := resolveDouyinRemoteDebuggingURL(context.Background(), secureServer.URL, secureServer.Client())
	if err != nil {
		t.Fatalf("resolveDouyinRemoteDebuggingURL() error = %v", err)
	}
	if !strings.HasPrefix(got, "wss://") {
		t.Fatalf("resolved endpoint = %q, want wss", got)
	}
}

func TestResolveDouyinRemoteDebuggingURLSupportsDefaultSecurePort(t *testing.T) {
	t.Parallel()

	requestedURL := make(chan string, 1)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestedURL <- request.URL.String()
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"webSocketDebuggerUrl":"wss://localhost/devtools/browser/fixture"}`)),
			Request:    request,
		}, nil
	})}
	got, err := resolveDouyinRemoteDebuggingURL(context.Background(), "https://localhost", client)
	if err != nil {
		t.Fatalf("resolveDouyinRemoteDebuggingURL() error = %v", err)
	}
	if rawURL := <-requestedURL; rawURL != "https://localhost/json/version" {
		t.Fatalf("discovery URL = %q", rawURL)
	}
	if got != "wss://localhost/devtools/browser/fixture" {
		t.Fatalf("resolved endpoint = %q", got)
	}
}

func TestResolveDouyinRemoteDebuggingURLDiscoversFromWebSocketBaseAddresses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		input        string
		discoveryURL string
		webSocketURL string
	}{
		{
			name:         "ws base",
			input:        "ws://localhost:9222/base",
			discoveryURL: "http://localhost:9222/json/version",
			webSocketURL: "ws://localhost:9222/devtools/browser/fixture",
		},
		{
			name:         "wss base without port",
			input:        "wss://localhost/base",
			discoveryURL: "https://localhost/json/version",
			webSocketURL: "wss://localhost/devtools/browser/fixture",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requestedURL := make(chan string, 1)
			client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				requestedURL <- request.URL.String()
				body := fmt.Sprintf(`{"webSocketDebuggerUrl":%q}`, test.webSocketURL)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
					Request:    request,
				}, nil
			})}
			got, err := resolveDouyinRemoteDebuggingURL(context.Background(), test.input, client)
			if err != nil {
				t.Fatalf("resolveDouyinRemoteDebuggingURL() error = %v", err)
			}
			if rawURL := <-requestedURL; rawURL != test.discoveryURL {
				t.Fatalf("discovery URL = %q, want %q", rawURL, test.discoveryURL)
			}
			if got != test.webSocketURL {
				t.Fatalf("resolved endpoint = %q, want %q", got, test.webSocketURL)
			}
		})
	}
}

func TestResolveDouyinRemoteDebuggingURLRejectsMalformedDiscoveryWithoutPanic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "malformed json", body: `{`},
		{name: "missing websocket url", body: `{}`},
		{name: "wrong websocket url type", body: `{"webSocketDebuggerUrl":42}`},
		{name: "non loopback websocket", body: `{"webSocketDebuggerUrl":"ws://example.com/devtools/browser/fixture"}`},
		{name: "non browser websocket", body: `{"webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/page/fixture"}`},
		{name: "oversized response", body: strings.Repeat("x", douyinCDPDiscoveryMaxBytes+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				_, _ = response.Write([]byte(test.body))
			}))
			defer server.Close()
			if _, err := resolveDouyinRemoteDebuggingURL(context.Background(), server.URL, server.Client()); err == nil {
				t.Fatal("resolveDouyinRemoteDebuggingURL() error = nil")
			}
		})
	}
}

func TestResolveDouyinRemoteDebuggingURLRejectsRedirectAndSecureDowngrade(t *testing.T) {
	t.Parallel()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, "/elsewhere", http.StatusFound)
	}))
	defer redirectServer.Close()
	if _, err := resolveDouyinRemoteDebuggingURL(context.Background(), redirectServer.URL, redirectServer.Client()); err == nil {
		t.Fatal("redirecting discovery endpoint was accepted")
	}

	secureServer := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"webSocketDebuggerUrl":"ws://127.0.0.1:9222/devtools/browser/fixture"}`))
	}))
	defer secureServer.Close()
	if _, err := resolveDouyinRemoteDebuggingURL(context.Background(), secureServer.URL, secureServer.Client()); err == nil {
		t.Fatal("secure discovery downgrade was accepted")
	}
}

func TestDouyinNetworkCaptureBoundsQueueAndResponseBody(t *testing.T) {
	t.Parallel()

	capture := &douyinNetworkCapture{
		ctx:     context.Background(),
		queue:   make(chan douyinNetworkResponse, douyinNetworkCaptureQueueSize),
		updated: make(chan struct{}, 1),
	}
	if got := cap(capture.queue); got != douyinNetworkCaptureQueueSize {
		t.Fatalf("capture queue capacity = %d, want %d", got, douyinNetworkCaptureQueueSize)
	}
	if capture.storeResponse(douyinNetworkResponsePoll, make([]byte, douyinNetworkResponseMaxBytes+1)) {
		t.Fatal("oversized response was stored")
	}
	if !capture.storeResponse(douyinNetworkResponsePoll, []byte(`{"data":{"status":"scanned"}}`)) {
		t.Fatal("bounded response was rejected")
	}
	body, sequence := capture.latestPoll()
	if len(body) == 0 || sequence != 1 {
		t.Fatalf("captured body length = %d, sequence = %d", len(body), sequence)
	}
}

func TestClassifyDouyinNetworkResponseRequiresExactHostAndPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url  string
		want douyinNetworkResponseKind
	}{
		{url: "https://www.douyin.com/passport/web/get_qrcode/?aid=6383", want: douyinNetworkResponseQRCode},
		{url: "https://login.douyin.com/passport/web/get_qrcode/?aid=6383", want: douyinNetworkResponseQRCode},
		{url: "https://login.douyin.com/passport/web/check_qrconnect/?token=fixture", want: douyinNetworkResponsePoll},
		{url: "https://sso.douyin.com/passport/web/check_qrconnect/?token=fixture", want: douyinNetworkResponsePoll},
		{url: "https://www.douyin.com.evil.test/passport/web/get_qrcode/", want: douyinNetworkResponseUnknown},
		{url: "https://www.douyin.com/passport/web/get_qrcode/extra", want: douyinNetworkResponseUnknown},
		{url: "http://www.douyin.com/passport/web/get_qrcode/", want: douyinNetworkResponseUnknown},
	}
	for _, tt := range tests {
		if got := classifyDouyinNetworkResponse(tt.url); got != tt.want {
			t.Fatalf("classifyDouyinNetworkResponse(%q) = %d, want %d", tt.url, got, tt.want)
		}
	}
}

func TestChromedpBrowserPollUsesStrongCookieAsAuthority(t *testing.T) {
	t.Parallel()

	browser := newPollTestBrowser(
		func(context.Context) (map[string]string, error) {
			return map[string]string{"sessionid": "fixture-session"}, nil
		},
		func(context.Context, context.Context) (douyinPageSignals, error) {
			return douyinPageSignals{}, nil
		},
		nil,
	)
	result, err := browser.Poll(context.Background(), "fixture-token")
	if err != nil {
		t.Fatalf("Poll returned error: %v", err)
	}
	if result.State != "succeeded" || result.Cookie == "" {
		t.Fatalf("Poll result = %#v, want succeeded with cookie", result)
	}
}

func TestChromedpBrowserAutoContinuesAfterAttemptSetupFailures(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 22, 8, 0, 0, 0, time.UTC)
	for _, stage := range []string{"navigation", "qrcode", "cookie"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			calls := 0
			browser := NewChromedpBrowser(BrowserOptions{
				Mode: BrowserModeAuto,
				attemptRunner: func(_ context.Context, attempt browserLaunchAttempt, _ time.Time) (BrowserCreateResult, *browserRuntime, error) {
					calls++
					if calls == 1 {
						return BrowserCreateResult{}, nil, fmt.Errorf("fixture %s failure", stage)
					}
					return BrowserCreateResult{
							Token:     "fixture-token",
							QRCodeURL: "https://example.test/fixture-qr",
							ExpiresAt: now.Add(time.Minute),
						}, &browserRuntime{
							ctx:       context.Background(),
							cancel:    func() {},
							expiresAt: now.Add(time.Minute),
							mode:      attempt.mode,
							capture:   &douyinNetworkCapture{},
							cookies:   map[string]string{},
							lastState: "pending_scan",
						}, nil
				},
			})
			result, err := browser.createWithAttempts(context.Background(), now, []browserLaunchAttempt{
				{mode: BrowserModeVisible},
				{mode: BrowserModeHeadless},
			})
			if err != nil {
				t.Fatalf("createWithAttempts() error = %v", err)
			}
			defer browser.Close(result.Token)
			if calls != 2 || result.Token != "fixture-token" {
				t.Fatalf("attempt calls = %d, result = %#v", calls, result)
			}
		})
	}
}

func TestChromedpBrowserAutoReservesDeadlineForFallbackAttempts(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 22, 8, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	parentDeadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("parent context has no deadline")
	}

	var attemptDeadlines []time.Time
	browser := NewChromedpBrowser(BrowserOptions{
		Mode: BrowserModeAuto,
		attemptRunner: func(attemptCtx context.Context, attempt browserLaunchAttempt, _ time.Time) (BrowserCreateResult, *browserRuntime, error) {
			deadline, ok := attemptCtx.Deadline()
			if !ok {
				t.Fatal("attempt context has no deadline")
			}
			attemptDeadlines = append(attemptDeadlines, deadline)
			if len(attemptDeadlines) == 1 {
				return BrowserCreateResult{}, nil, errors.New("fixture setup timeout")
			}
			return BrowserCreateResult{
					Token:     "fixture-token",
					QRCodeURL: "https://example.test/fixture-qr",
					ExpiresAt: now.Add(time.Minute),
				}, &browserRuntime{
					ctx:       context.Background(),
					cancel:    func() {},
					expiresAt: now.Add(time.Minute),
					mode:      attempt.mode,
					capture:   &douyinNetworkCapture{},
					cookies:   map[string]string{},
					lastState: "pending_scan",
				}, nil
		},
	})
	result, err := browser.createWithAttempts(ctx, now, []browserLaunchAttempt{
		{mode: BrowserModeVisible},
		{mode: BrowserModeHeadless},
	})
	if err != nil {
		t.Fatalf("createWithAttempts() error = %v", err)
	}
	defer browser.Close(result.Token)
	if len(attemptDeadlines) != 2 {
		t.Fatalf("attempt deadlines = %d, want 2", len(attemptDeadlines))
	}
	if !attemptDeadlines[0].Before(parentDeadline) {
		t.Fatalf("first attempt deadline = %s, parent deadline = %s", attemptDeadlines[0], parentDeadline)
	}
	if !attemptDeadlines[1].Equal(parentDeadline) {
		t.Fatalf("fallback deadline = %s, want parent deadline %s", attemptDeadlines[1], parentDeadline)
	}
}

func TestChromedpBrowserPollRequestCancellationDoesNotFailSession(t *testing.T) {
	t.Parallel()

	firstSignalRead := true
	var cancelRequest context.CancelFunc
	browser := newPollTestBrowser(
		func(context.Context) (map[string]string, error) { return map[string]string{}, nil },
		func(requestCtx context.Context, _ context.Context) (douyinPageSignals, error) {
			if firstSignalRead {
				firstSignalRead = false
				cancelRequest()
				return douyinPageSignals{}, requestCtx.Err()
			}
			return douyinPageSignals{}, nil
		},
		nil,
	)
	requestCtx, cancel := context.WithCancel(context.Background())
	cancelRequest = cancel
	if _, err := browser.Poll(requestCtx, "fixture-token"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Poll error = %v, want context.Canceled", err)
	}
	result, err := browser.Poll(context.Background(), "fixture-token")
	if err != nil {
		t.Fatalf("second Poll returned error: %v", err)
	}
	if result.State != "pending_scan" {
		t.Fatalf("second Poll state = %q, want pending_scan", result.State)
	}
}

func TestIsDouyinBrowserContextLostDoesNotTreatRequestCancellationAsSessionLoss(t *testing.T) {
	t.Parallel()

	for _, err := range []error{context.Canceled, errors.New("context canceled")} {
		if isDouyinBrowserContextLost(err) {
			t.Fatalf("request cancellation %v was classified as browser context loss", err)
		}
	}
	for _, err := range []error{chromedp.ErrInvalidContext, errors.New("target closed")} {
		if !isDouyinBrowserContextLost(err) {
			t.Fatalf("browser failure %v was not classified as context loss", err)
		}
	}
}

func TestChromedpBrowserFallbackPollsDisplayedTokenAndFollowsRedirect(t *testing.T) {
	t.Parallel()

	var polledToken string
	var followedRedirect string
	cookieReady := false
	browser := NewChromedpBrowser(BrowserOptions{
		cookieReader: func(context.Context) (map[string]string, error) {
			if cookieReady {
				return map[string]string{"sessionid": "fixture-session"}, nil
			}
			return map[string]string{}, nil
		},
		signalReader: func(context.Context, context.Context) (douyinPageSignals, error) {
			return douyinPageSignals{}, nil
		},
		fallbackPoller: func(_ context.Context, _ context.Context, token string) (douyinBrowserPollResponse, error) {
			polledToken = token
			return douyinBrowserPollResponse{
				State:       "succeeded",
				RedirectURL: "https://www.douyin.com/passport/sso/login/callback/?ticket=fixture",
			}, nil
		},
		redirectFollower: func(_ context.Context, _ context.Context, rawURL string) error {
			followedRedirect = rawURL
			cookieReady = true
			return nil
		},
	})
	browser.sessions["displayed-token"] = &browserRuntime{
		ctx:       context.Background(),
		cancel:    func() {},
		expiresAt: time.Now().Add(time.Minute),
		mode:      BrowserModeHeadless,
		capture:   &douyinNetworkCapture{},
		pollMode:  browserQRCodePollActiveToken,
		cookies:   map[string]string{},
		lastState: "pending_scan",
	}

	result, err := browser.Poll(context.Background(), "displayed-token")
	if err != nil {
		t.Fatalf("Poll returned error: %v", err)
	}
	if polledToken != "displayed-token" {
		t.Fatalf("polled token = %q, want displayed-token", polledToken)
	}
	if followedRedirect == "" || result.State != "succeeded" || result.Cookie == "" {
		t.Fatalf("fallback poll result = %#v, redirect = %q", result, followedRedirect)
	}
}

func TestChromedpBrowserFallbackIgnoresCapturedStateForAnotherToken(t *testing.T) {
	t.Parallel()

	browser := NewChromedpBrowser(BrowserOptions{
		cookieReader: func(context.Context) (map[string]string, error) { return map[string]string{}, nil },
		signalReader: func(context.Context, context.Context) (douyinPageSignals, error) {
			return douyinPageSignals{}, nil
		},
		fallbackPoller: func(_ context.Context, _ context.Context, token string) (douyinBrowserPollResponse, error) {
			if token != "displayed-token" {
				t.Fatalf("fallback polled token = %q", token)
			}
			return douyinBrowserPollResponse{State: "pending_scan"}, nil
		},
	})
	capture := &douyinNetworkCapture{
		lastPoll: []byte(`{"data":{"error_code":0,"status":"succeeded"},"message":"success"}`),
		pollSeq:  1,
	}
	browser.sessions["displayed-token"] = &browserRuntime{
		ctx:       context.Background(),
		cancel:    func() {},
		expiresAt: time.Now().Add(time.Minute),
		mode:      BrowserModeHeadless,
		capture:   capture,
		pollMode:  browserQRCodePollActiveToken,
		cookies:   map[string]string{},
		lastState: "pending_scan",
	}

	result, err := browser.Poll(context.Background(), "displayed-token")
	if err != nil {
		t.Fatalf("Poll returned error: %v", err)
	}
	if result.State != "pending_scan" {
		t.Fatalf("state = %q, want pending_scan", result.State)
	}
}

func TestChromedpBrowserPollDoesNotTrustLoginMarkersAlone(t *testing.T) {
	t.Parallel()

	browser := newPollTestBrowser(
		func(context.Context) (map[string]string, error) {
			return map[string]string{"LOGIN_STATUS": "1"}, nil
		},
		func(context.Context, context.Context) (douyinPageSignals, error) {
			return douyinPageSignals{HasUserLogin: true}, nil
		},
		nil,
	)
	result, err := browser.Poll(context.Background(), "fixture-token")
	if err != nil {
		t.Fatalf("Poll returned error: %v", err)
	}
	if result.State != "pending_confirm" || result.Cookie != "" {
		t.Fatalf("Poll result = %#v, want pending_confirm without credential", result)
	}
}

func TestChromedpBrowserPollReportsVerificationAndRiskFailure(t *testing.T) {
	t.Parallel()

	scanned := []byte(`{"data":{"error_code":0,"status":"scanned"},"message":"success"}`)
	browser := newPollTestBrowser(
		func(context.Context) (map[string]string, error) { return map[string]string{}, nil },
		func(context.Context, context.Context) (douyinPageSignals, error) {
			return douyinPageSignals{VerificationRequired: true}, nil
		},
		scanned,
	)
	result, err := browser.Poll(context.Background(), "fixture-token")
	if err != nil {
		t.Fatalf("Poll returned error: %v", err)
	}
	if result.State != "verification_required" {
		t.Fatalf("state = %q, want verification_required", result.State)
	}

	blocked := []byte(`{"data":{"error_code":1105,"description":"安全风险，已阻止此次访问。"},"message":"error"}`)
	browser = newPollTestBrowser(
		func(context.Context) (map[string]string, error) { return map[string]string{}, nil },
		func(context.Context, context.Context) (douyinPageSignals, error) { return douyinPageSignals{}, nil },
		blocked,
	)
	for attempt, want := range []string{"pending_scan", "pending_scan"} {
		result, err = browser.Poll(context.Background(), "fixture-token")
		if err != nil {
			t.Fatalf("Poll attempt %d returned error: %v", attempt+1, err)
		}
		if result.State != want {
			t.Fatalf("Poll attempt %d state = %q, want %q", attempt+1, result.State, want)
		}
	}
	nonRiskFailure := []byte(`{"data":{"error_code":1001,"description":"temporary error"},"message":"error"}`)
	if !browser.sessions["fixture-token"].capture.storeResponse(douyinNetworkResponsePoll, nonRiskFailure) {
		t.Fatal("non-risk response was not stored")
	}
	if _, err = browser.Poll(context.Background(), "fixture-token"); err == nil {
		t.Fatal("Poll after non-risk response returned nil error")
	}
	if !browser.sessions["fixture-token"].capture.storeResponse(douyinNetworkResponsePoll, blocked) {
		t.Fatal("risk response after reset was not stored")
	}
	result, err = browser.Poll(context.Background(), "fixture-token")
	if err != nil {
		t.Fatalf("Poll after reset risk response returned error: %v", err)
	}
	if result.State != "pending_scan" {
		t.Fatalf("state after reset risk response = %q, want pending_scan", result.State)
	}
	if !browser.sessions["fixture-token"].capture.storeResponse(douyinNetworkResponsePoll, blocked) {
		t.Fatal("second risk response was not stored")
	}
	result, err = browser.Poll(context.Background(), "fixture-token")
	if err != nil {
		t.Fatalf("Poll after second risk response returned error: %v", err)
	}
	if result.State != "failed" {
		t.Fatalf("state after two distinct risk responses = %q, want failed", result.State)
	}
}

func TestAdvanceDouyinBrowserStateDoesNotRegress(t *testing.T) {
	t.Parallel()

	if got := advanceDouyinBrowserState("verification_required", "pending_scan"); got != "verification_required" {
		t.Fatalf("state regressed to %q", got)
	}
	if got := advanceDouyinBrowserState("pending_confirm", "pending_scan"); got != "pending_confirm" {
		t.Fatalf("state regressed to %q", got)
	}
}

func newPollTestBrowser(
	cookieReader func(context.Context) (map[string]string, error),
	signalReader func(context.Context, context.Context) (douyinPageSignals, error),
	pollBody []byte,
) *ChromedpBrowser {
	browser := NewChromedpBrowser(BrowserOptions{cookieReader: cookieReader, signalReader: signalReader})
	capture := &douyinNetworkCapture{}
	capture.lastPoll = append([]byte(nil), pollBody...)
	if len(pollBody) > 0 {
		capture.pollSeq = 1
	}
	browser.sessions["fixture-token"] = &browserRuntime{
		ctx:       context.Background(),
		cancel:    func() {},
		expiresAt: time.Now().Add(time.Minute),
		mode:      BrowserModeHeadless,
		capture:   capture,
		cookies:   map[string]string{},
		lastState: "pending_scan",
	}
	return browser
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
