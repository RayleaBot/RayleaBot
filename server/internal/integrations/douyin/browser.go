package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const douyinBrowserLoginURL = "https://www.douyin.com/login_page?service=https%3A%2F%2Fwww.douyin.com%2F"

type ChromedpBrowser struct {
	browserPath string
	browserArgs []string
	client      *http.Client

	mu       sync.Mutex
	sessions map[string]*browserRuntime
}

type browserRuntime struct {
	ctx       context.Context
	cancel    context.CancelFunc
	expiresAt time.Time
	cookies   map[string]string
}

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

func (b *ChromedpBrowser) Create(ctx context.Context, now time.Time) (BrowserCreateResult, error) {
	tabCtx, cancel := newDouyinBrowserContext(b.browserPath, b.browserArgs)
	runCtx, cancelRun := context.WithTimeout(tabCtx, 30*time.Second)
	defer cancelRun()

	// Navigate to douyin login page, wait for the QR code image to render,
	// then extract it from the DOM. No network listener or fetch() needed.
	var qrResult string

	err := chromedp.Run(runCtx,
		network.Enable(),
		chromedp.Navigate(douyinBrowserLoginURL),
		chromedp.WaitReady("body"),
		chromedp.Evaluate(`Object.defineProperty(navigator, 'webdriver', {get: () => undefined})`, nil),
		chromedp.Evaluate(`window.chrome = {runtime: {}, loadTimes: function(){}, csi: function(){}, app: {}}`, nil),
		chromedp.Evaluate(`delete window.__nightmare`, nil),
		// Wait for the QR code image to be rendered by the page's own JS.
		chromedp.WaitVisible(`img[src*="sso"]`, chromedp.ByQuery),
		chromedp.Evaluate(`(function(){
			var imgs = document.querySelectorAll('img');
			for (var i = 0; i < imgs.length; i++) {
				var s = imgs[i].src || '';
				if (s.indexOf('sso') !== -1 || s.indexOf('qrcode') !== -1) {
					var t = '';
					var m = s.match(/[?&]token=([^&]+)/i);
					if (m) t = m[1];
					if (!t) t = imgs[i].getAttribute('data-token') || '';
					if (!t) { var m2 = s.match(/[?&]qrcode_key=([^&]+)/i); if (m2) t = m2[1]; }
					return JSON.stringify({src: s, token: t});
				}
			}
			return JSON.stringify({src: '', token: ''});
		})()`, &qrResult),
	)
	if err != nil {
		cancel()
		return BrowserCreateResult{}, fmt.Errorf("douyin browser: %w", err)
	}

	var extracted struct {
		Src   string `json:"src"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(qrResult), &extracted); err != nil {
		cancel()
		return BrowserCreateResult{}, fmt.Errorf("douyin browser: parse DOM result: %w (raw: %s)", err, qrResult)
	}

	token := strings.TrimSpace(extracted.Token)
	qrcodeURL := strings.TrimSpace(extracted.Src)
	if token == "" || qrcodeURL == "" {
		cancel()
		return BrowserCreateResult{}, fmt.Errorf("douyin browser: QR code not found on page (token=%q, src=%q)", token, qrcodeURL)
	}

	cookies, err := b.cookies(tabCtx)
	if err != nil {
		cancel()
		return BrowserCreateResult{}, err
	}

	result := BrowserCreateResult{
		Token:     token,
		QRCodeURL: qrcodeURL,
		ExpiresAt: now.Add(3 * time.Minute),
	}

	b.mu.Lock()
	if old := b.sessions[token]; old != nil {
		old.cancel()
	}
	b.sessions[token] = &browserRuntime{
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

	cookies := cloneStringMap(session.cookies)
	result, err := pollDouyinQRCode(ctx, b.client, time.Now().UTC(), token, cookies)
	if err != nil {
		return BrowserPollResult{}, err
	}
	if result.State == "succeeded" && !HasLoginCookie(cookies) {
		if browserCookies, err := b.waitLoginCookies(session.ctx, 5*time.Second); err == nil {
			for key, value := range browserCookies {
				cookies[key] = value
			}
			result.Cookie = cookieHeaderFromMap(cookies)
			result.Cookies = cloneStringMap(cookies)
		}
	}
	session.cookies = cloneStringMap(cookies)
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
	allocatorOptions := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	allocatorOptions = append(allocatorOptions,
		chromedp.NoDefaultBrowserCheck,
		chromedp.NoFirstRun,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-features", "IsolateOrigins,site-per-process"),
		chromedp.Flag("disable-site-isolation-trials", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("window-size", "1920,1080"),
	)
	path := strings.TrimSpace(browserPath)
	if path == "" {
		// Auto-detect Chrome on Windows.
		for _, candidate := range []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		} {
			path = candidate
			break // use first candidate; chromedp.ExecPath overwrites previous
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
