package douyin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	douyinBrowserLoginURL      = "https://www.douyin.com/login_page?service=https%3A%2F%2Fwww.douyin.com%2F"
	douyinBrowserQRCodePath    = "/passport/web/get_qrcode/"
	douyinBrowserQRConnectPath = "/passport/web/check_qrconnect/"
)

type ChromedpBrowser struct {
	browserPath string
	browserArgs []string
	client      *http.Client

	mu       sync.Mutex
	sessions map[string]*browserRuntime
}

type browserRuntime struct {
	ctx        context.Context
	cancel     context.CancelFunc
	expiresAt  time.Time
	cookies    map[string]string
	blockCount int
	lastState  string // track last non-pending_scan state across page redirects
	mu         sync.Mutex
}

var errDouyinQRCodePollBlocked = errors.New("douyin qrcode poll blocked by risk control")

func NewChromedpBrowser(browserPath string, browserArgs []string, transport http.RoundTripper) *ChromedpBrowser {
	return &ChromedpBrowser{
		browserPath: strings.TrimSpace(browserPath),
		browserArgs: append([]string(nil), browserArgs...),
		client:      commonHTTPClient(transport),
		sessions:    make(map[string]*browserRuntime),
	}
}

func commonHTTPClient(transport http.RoundTripper) *http.Client {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// captureScript creates the state object AND intercepts fetch/XHR to
// passively capture QR code API responses. The wrappers are carefully
// crafted to match native function signatures so bdms.js cannot detect them:
//   - fetch.length === 1 (single `input` parameter)
//   - fetch.prototype is deleted (native fetch has no prototype)
//   - XMLHttpRequest.prototype === native _XHR.prototype
//   - toString() returns [native code] for both
// No browser prototypes (Canvas, WebGL, navigator, screen) are touched.
const captureScript = `(function(){
if (window.__rayleaDouyinLogin) return;

var state = {qrcode:null, lastPoll:null};
Object.defineProperty(window, '__rayleaDouyinLogin', {value:state, configurable:false, writable:false});

var _fetch = window.fetch;
var _XHR = window.XMLHttpRequest;

function match(url, path){ return typeof url==='string' && url.indexOf(path)!==-1; }
function capture(url, text){
if (!text) return;
try {
var p = JSON.parse(text);
if (match(url, '` + douyinBrowserQRCodePath + `')) state.qrcode = p;
if (match(url, '` + douyinBrowserQRConnectPath + `')) state.lastPoll = p;
} catch(e) {}
}

// Fetch wrapper — length=1, no prototype (matches native fetch).
window.fetch = function fetch(input){
var url = typeof input==='string' ? input : (input && input.url) || '';
return _fetch.call(this, input, arguments[1]).then(function(r){
if (match(url, '` + douyinBrowserQRCodePath + `') || match(url, '` + douyinBrowserQRConnectPath + `')){
try { r.clone().text().then(function(t){ capture(url, t); }).catch(function(){}); } catch(e) {}
}
return r;
});
};
try { delete window.fetch.prototype; } catch(e) {}
window.fetch.toString = function(){ return 'function fetch() { [native code] }'; };

// XMLHttpRequest wrapper — inherits native prototype.
var xhrWrapper = function XMLHttpRequest(){
var x = new _XHR();
var url = '';
var _open = x.open;
x.open = function(m, u){ url = String(u || ''); return _open.apply(x, arguments); };
x.addEventListener('load', function(){
if (match(url, '` + douyinBrowserQRCodePath + `') || match(url, '` + douyinBrowserQRConnectPath + `')){
capture(url, x.responseText || '');
}
});
return x;
};
xhrWrapper.prototype = _XHR.prototype;
xhrWrapper.toString = function(){ return 'function XMLHttpRequest() { [native code] }'; };
window.XMLHttpRequest = xhrWrapper;

// Critical: make Function.prototype.toString.call(fetch) also return [native code].
// bdms.js uses this to verify that fetch/XHR are the real native functions.
var _funcToString = Function.prototype.toString;
Function.prototype.toString = function toString(){
if (this === window.fetch || this === window.XMLHttpRequest) {
return this.toString();
}
return _funcToString.call(this);
};
Function.prototype.toString.toString = function(){ return 'function toString() { [native code] }'; };
})();`

func installCaptureScript(ctx context.Context) error {
	_, err := page.AddScriptToEvaluateOnNewDocument(captureScript).Do(ctx)
	return err
}

// callQRCodeAPI actively calls Douyin's get_qrcode API from within the page
// context. If the security SDK (bdms.js) intercepts fetch to add signatures,
// this will succeed. Otherwise, fall back to passive interception.
func callQRCodeAPI(ctx context.Context) (json.RawMessage, error) {
	js := `(function(){
var u = '/passport/web/get_qrcode/?aid=6383&service=' + encodeURIComponent('https://www.douyin.com/') + '&need_logo=true&t=' + Date.now();
return fetch(u, {credentials: 'include'}).then(function(r){ return r.text(); }).then(function(t){
var d = JSON.parse(t);
if (d && d.data) { window.__rayleaDouyinLogin.qrcode = d; }
return t;
}).catch(function(e){ return '{"error":"'+e.message+'"}'; });
})()`

	var raw string
	if err := chromedp.Evaluate(js, &raw, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}).Do(ctx); err != nil {
		return nil, err
	}
	var check struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &check); err == nil && check.Error != "" {
		return nil, fmt.Errorf("douyin qrcode api call: %s", check.Error)
	}
	return json.RawMessage(raw), nil
}

// readQRCodeFromState reads the QR code data from the injected JS state.
// This works whether the data was set by active API calls or passive interception.
func readQRCodeFromState(ctx context.Context) (string, error) {
	var raw string
	if err := chromedp.Evaluate(`(function(){var s=window.__rayleaDouyinLogin; return s && s.qrcode ? JSON.stringify(s.qrcode) : "";})()`, &raw).Do(ctx); err != nil {
		return "", err
	}
	return raw, nil
}

// readPollFromState reads the latest poll response from the injected JS state.
func readPollFromState(ctx context.Context) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var raw string
	if err := chromedp.Run(runCtx, chromedp.Evaluate(`(function(){var s=window.__rayleaDouyinLogin; return s && s.lastPoll ? JSON.stringify(s.lastPoll) : "";})()`, &raw)); err != nil {
		return "", err
	}
	return raw, nil
}

func waitDouyinBrowserQRCode(ctx context.Context, now time.Time) (BrowserCreateResult, error) {
	// First, try passive — the page may have already called get_qrcode.
	// Wait briefly for the page's security SDKs to finish initialization.
	initialWait := 3 * time.Second
	select {
	case <-ctx.Done():
		return BrowserCreateResult{}, ctx.Err()
	case <-time.After(initialWait):
	}

	// Check if the page already has QR code data (passive interception).
	if raw, err := readQRCodeFromState(ctx); err == nil && strings.TrimSpace(raw) != "" {
		return parseDouyinBrowserQRCodeResponse([]byte(raw), now)
	}

	// Try active API call.
	for attempt := 0; attempt < 5; attempt++ {
		raw, err := callQRCodeAPI(ctx)
		if err == nil {
			return parseDouyinBrowserQRCodeResponse(raw, now)
		}
		// If the API returns an error (missing signatures), wait and retry.
		select {
		case <-ctx.Done():
			return BrowserCreateResult{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	// Last check: page might have made the call despite our active attempts failing.
	if raw, err := readQRCodeFromState(ctx); err == nil && strings.TrimSpace(raw) != "" {
		return parseDouyinBrowserQRCodeResponse([]byte(raw), now)
	}

	return BrowserCreateResult{}, fmt.Errorf("douyin browser: unable to obtain QR code after %v", time.Since(now))
}

func readDouyinBrowserPollState(ctx context.Context) (string, error) {
	// Check passive interception first.
	raw, err := readPollFromState(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(raw) == "" {
		return "pending_scan", nil
	}
	return parseDouyinBrowserPollState([]byte(raw))
}

func parseDouyinBrowserQRCodeResponse(body []byte, now time.Time) (BrowserCreateResult, error) {
	var response struct {
		Message string `json:"message"`
		Data    struct {
			ErrorCode      int    `json:"error_code"`
			Description    string `json:"description"`
			QRCodeIndexURL string `json:"qrcode_index_url"`
			Token          string `json:"token"`
			ExpireTime     int64  `json:"expire_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return BrowserCreateResult{}, fmt.Errorf("douyin browser qrcode response: %w", err)
	}
	if response.Data.ErrorCode != 0 {
		return BrowserCreateResult{}, fmt.Errorf("douyin browser qrcode create failed: %s", firstNonEmpty(response.Data.Description, response.Message, "invalid response"))
	}
	token := strings.TrimSpace(response.Data.Token)
	qrcodeURL := strings.TrimSpace(response.Data.QRCodeIndexURL)
	if token == "" || qrcodeURL == "" {
		return BrowserCreateResult{}, fmt.Errorf("douyin browser qrcode create missing token or qrcode url")
	}
	expiresAt := now.Add(3 * time.Minute)
	if response.Data.ExpireTime > 0 {
		remoteExpiresAt := time.Unix(response.Data.ExpireTime, 0).UTC()
		if remoteExpiresAt.After(now) {
			expiresAt = remoteExpiresAt
		}
	}
	return BrowserCreateResult{
		Token:     token,
		QRCodeURL: qrcodeURL,
		ExpiresAt: expiresAt,
	}, nil
}

func parseDouyinBrowserPollState(body []byte) (string, error) {
	var response struct {
		Message string `json:"message"`
		Data    struct {
			ErrorCode   int             `json:"error_code"`
			Description string          `json:"description"`
			Status      json.RawMessage `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("douyin browser qrcode poll response: %w", err)
	}
	if response.Data.ErrorCode != 0 {
		message := firstNonEmpty(response.Data.Description, response.Message, "invalid response")
		if isDouyinQRCodePollBlocked(message) {
			return "", fmt.Errorf("%w: %s", errDouyinQRCodePollBlocked, message)
		}
		return "", fmt.Errorf("douyin browser qrcode poll failed: %s", message)
	}
	switch douyinStatus(response.Data.Status) {
	case "", "1", "new":
		return "pending_scan", nil
	case "2", "scan", "scanned":
		return "pending_confirm", nil
	case "3", "confirm", "confirmed", "success", "succeeded":
		return "succeeded", nil
	case "4", "5", "expire", "expired", "cancel", "canceled", "cancelled":
		return "expired", nil
	default:
		return "", fmt.Errorf("douyin browser qrcode poll status %s", string(response.Data.Status))
	}
}

func isDouyinQRCodePollBlocked(message string) bool {
	message = strings.TrimSpace(message)
	return strings.Contains(message, "安全风险") || strings.Contains(message, "已阻止此次访问")
}

func (b *ChromedpBrowser) Create(ctx context.Context, now time.Time) (BrowserCreateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return BrowserCreateResult{}, fmt.Errorf("douyin browser: %w", err)
	}
	var result BrowserCreateResult
	tabCtx, cancel := newDouyinBrowserContext(b.browserPath, b.browserArgs)
	errCh := make(chan error, 1)
	go func() {
		errCh <- chromedp.Run(tabCtx,
			network.Enable(),
			emulation.SetTimezoneOverride("Asia/Shanghai"),
			emulation.SetFocusEmulationEnabled(true),
			chromedp.ActionFunc(installCaptureScript),
			chromedp.Navigate(douyinBrowserLoginURL),
			chromedp.WaitReady("body"),
			chromedp.ActionFunc(func(ctx context.Context) error {
				created, err := waitDouyinBrowserQRCode(ctx, now)
				if err != nil {
					return err
				}
				result = created
				return nil
			}),
		)
	}()
	timer := time.NewTimer(douyinBrowserCreateTimeout)
	defer timer.Stop()
	var err error
	select {
	case err = <-errCh:
	case <-ctx.Done():
		cancel()
		return BrowserCreateResult{}, fmt.Errorf("douyin browser: %w", ctx.Err())
	case <-timer.C:
		cancel()
		return BrowserCreateResult{}, fmt.Errorf("douyin browser: %w", context.DeadlineExceeded)
	}
	if err != nil {
		cancel()
		return BrowserCreateResult{}, fmt.Errorf("douyin browser: %w", err)
	}

	cookies, err := b.cookies(tabCtx)
	if err != nil {
		cancel()
		return BrowserCreateResult{}, err
	}

	b.mu.Lock()
	if old := b.sessions[result.Token]; old != nil {
		old.cancel()
	}
	b.sessions[result.Token] = &browserRuntime{
		ctx:       tabCtx,
		cancel:    cancel,
		expiresAt: result.ExpiresAt,
		cookies:   cloneStringMap(cookies),
	}
	b.mu.Unlock()

	result.Cookies = cloneStringMap(cookies)
	return result, nil
}

func (b *ChromedpBrowser) Poll(ctx context.Context, token string) (BrowserPollResult, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return BrowserPollResult{}, fmt.Errorf("douyin browser poll: login session not found")
	}
	session := b.session(token)
	if session == nil {
		return BrowserPollResult{}, fmt.Errorf("douyin browser poll: login session not found")
	}
	if !session.expiresAt.IsZero() && time.Now().UTC().After(session.expiresAt) {
		return BrowserPollResult{State: "expired"}, nil
	}

	state, err := readDouyinBrowserPollState(session.ctx)
	if err != nil {
		if errors.Is(err, errDouyinQRCodePollBlocked) {
			session.mu.Lock()
			session.blockCount++
			blocked := session.blockCount
			session.mu.Unlock()
			if blocked < 2 {
				return BrowserPollResult{State: "pending_scan"}, nil
			}
		}
		return BrowserPollResult{}, err
	}
	session.mu.Lock()
	session.blockCount = 0
	// Track the last meaningful state across page redirects.
	// When Douyin completes login, the page redirects to a callback URL
	// which resets our JS state. We preserve the state progression here.
	if state != "pending_scan" && state != "" {
		session.lastState = state
	}
	prevState := session.lastState
	session.mu.Unlock()

	if state == "" {
		state = "pending_scan"
	}

	// Detect page redirect after scan: if we previously saw pending_confirm
	// but now see pending_scan, the page likely redirected after success.
	// Check for login cookies immediately.
	if state == "pending_scan" && (prevState == "pending_confirm" || prevState == "succeeded") {
		session.mu.Lock()
		cookies := cloneStringMap(session.cookies)
		session.mu.Unlock()
		if browserCookies, err := b.waitLoginCookies(session.ctx, 3*time.Second); err == nil {
			for key, value := range browserCookies {
				cookies[key] = value
			}
		}
		if HasLoginCookie(cookies) {
			state = "succeeded"
			session.mu.Lock()
			session.cookies = cloneStringMap(cookies)
			session.mu.Unlock()
			return BrowserPollResult{
				State:   state,
				Cookie:  cookieHeaderFromMap(cookies),
				Cookies: cloneStringMap(cookies),
			}, nil
		}
	}

	session.mu.Lock()
	cookies := cloneStringMap(session.cookies)
	session.mu.Unlock()
	if state == "succeeded" {
		if browserCookies, err := b.waitLoginCookies(session.ctx, 5*time.Second); err == nil {
			for key, value := range browserCookies {
				cookies[key] = value
			}
		}
		if !HasLoginCookie(cookies) {
			return BrowserPollResult{}, fmt.Errorf("douyin qrcode login succeeded without login cookie")
		}
	}
	result := BrowserPollResult{
		State:   state,
		Cookie:  cookieHeaderFromMap(cookies),
		Cookies: cloneStringMap(cookies),
	}
	if state != "succeeded" {
		result.Cookie = ""
	}
	session.mu.Lock()
	session.cookies = cloneStringMap(cookies)
	session.mu.Unlock()
	return result, nil
}

func (b *ChromedpBrowser) Close(token string) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	b.mu.Lock()
	session := b.sessions[token]
	delete(b.sessions, token)
	b.mu.Unlock()
	if session != nil {
		session.cancel()
	}
}

// SessionContext returns the browser tab context for the given session token,
// or nil if the session is not found. This allows profile fetching from the
// browser context after login succeeds.
func (b *ChromedpBrowser) SessionContext(token string) context.Context {
	s := b.session(token)
	if s == nil {
		return nil
	}
	return s.ctx
}

func (b *ChromedpBrowser) session(token string) *browserRuntime {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sessions[token]
}

func (b *ChromedpBrowser) cookies(ctx context.Context) (map[string]string, error) {
	var values map[string]string
	runCtx, cancelRun := context.WithTimeout(ctx, 10*time.Second)
	defer cancelRun()
	err := chromedp.Run(runCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		cookies, err := network.GetCookies().WithURLs([]string{
			"https://www.douyin.com/",
			"https://login.douyin.com/",
			"https://sso.douyin.com/",
			"https://api.amemv.com/",
		}).Do(ctx)
		if err != nil {
			return err
		}
		values = douyinNetworkCookies(cookies)
		return nil
	}))
	return values, err
}

func (b *ChromedpBrowser) waitLoginCookies(ctx context.Context, timeout time.Duration) (map[string]string, error) {
	deadline := time.Now().Add(timeout)
	var last map[string]string
	for {
		cookies, err := b.cookies(ctx)
		if err != nil {
			return nil, err
		}
		last = cookies
		if HasLoginCookie(cookies) || time.Now().After(deadline) {
			return last, nil
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func newDouyinBrowserContext(browserPath string, browserArgs []string) (context.Context, context.CancelFunc) {
	allocatorOptions := []chromedp.ExecAllocatorOption{
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,

		chromedp.Flag("headless", "new"),

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
	}
	path := strings.TrimSpace(browserPath)
	if path == "" {
		for _, candidate := range []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		} {
			path = candidate
			break
		}
	}
	if path != "" {
		allocatorOptions = append(allocatorOptions, chromedp.ExecPath(path))
	}
	allocatorOptions = append(allocatorOptions, douyinAllocatorFlags(browserArgs)...)
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), allocatorOptions...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	tabCtx, cancelTab := chromedp.NewContext(browserCtx)
	return tabCtx, func() {
		cancelTab()
		cancelBrowser()
		cancelAllocator()
	}
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
		if strings.Contains(domain, "douyin.com") || strings.Contains(domain, "amemv.com") || strings.Contains(domain, "bytedance.com") {
			values[name] = value
		}
	}
	return values
}

func douyinAllocatorFlags(arguments []string) []chromedp.ExecAllocatorOption {
	flags := make([]chromedp.ExecAllocatorOption, 0, len(arguments))
	for _, argument := range arguments {
		argument = strings.TrimSpace(argument)
		argument = strings.TrimPrefix(argument, "--")
		if argument == "" {
			continue
		}
		if key, value, ok := strings.Cut(argument, "="); ok {
			flags = append(flags, chromedp.Flag(key, value))
			continue
		}
		flags = append(flags, chromedp.Flag(argument, true))
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
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
