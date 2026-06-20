package thirdpartylogin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

const (
	douyinBrowserLoginURL    = "https://www.douyin.com/login_page?service=https%3A%2F%2Fwww.douyin.com%2F"
	douyinBrowserCreateMatch = "/passport/web/get_qrcode/"
	douyinBrowserPollMatch   = "/passport/web/check_qrconnect/"
)

type douyinBrowserOptions struct {
	BrowserPath string
	BrowserArgs []string
}

type chromedpDouyinBrowser struct {
	browserPath string
	browserArgs []string

	mu       sync.Mutex
	sessions map[string]*douyinBrowserRuntime
}

type douyinBrowserRuntime struct {
	ctx       context.Context
	cancel    context.CancelFunc
	expiresAt time.Time
}

func newChromedpDouyinBrowser(options douyinBrowserOptions) *chromedpDouyinBrowser {
	return &chromedpDouyinBrowser{
		browserPath: strings.TrimSpace(options.BrowserPath),
		browserArgs: append([]string(nil), options.BrowserArgs...),
		sessions:    make(map[string]*douyinBrowserRuntime),
	}
}

func (b *chromedpDouyinBrowser) Create(ctx context.Context, now time.Time) (douyinBrowserCreateResult, error) {
	tabCtx, cancel := newDouyinBrowserContext(b.browserPath, b.browserArgs)
	runCtx, cancelRun := context.WithTimeout(tabCtx, 35*time.Second)
	defer cancelRun()

	var body string
	err := chromedp.Run(runCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(douyinBrowserCaptureScript).Do(ctx)
			return err
		}),
		network.Enable(),
		chromedp.Navigate(douyinBrowserLoginURL),
		chromedp.WaitReady("body"),
		waitDouyinCapturedBody(douyinBrowserCreateMatch, &body),
	)
	if err != nil {
		cancel()
		return douyinBrowserCreateResult{}, err
	}

	var response struct {
		Message string `json:"message"`
		Data    struct {
			ErrorCode      int    `json:"error_code"`
			Description    string `json:"description"`
			Token          string `json:"token"`
			QRCodeIndexURL string `json:"qrcode_index_url"`
			ExpireTime     int64  `json:"expire_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		cancel()
		return douyinBrowserCreateResult{}, fmt.Errorf("decode douyin qrcode create response: %w", err)
	}
	if response.Data.ErrorCode != 0 {
		cancel()
		return douyinBrowserCreateResult{}, fmt.Errorf("douyin qrcode create failed: %s", firstNonEmpty(response.Data.Description, response.Message, "invalid response"))
	}
	token := strings.TrimSpace(response.Data.Token)
	qrcodeURL := strings.TrimSpace(response.Data.QRCodeIndexURL)
	if token == "" || qrcodeURL == "" {
		cancel()
		return douyinBrowserCreateResult{}, fmt.Errorf("douyin qrcode create missing token or qrcode url")
	}

	expiresAt := now.Add(3 * time.Minute)
	if response.Data.ExpireTime > 0 {
		remoteExpiresAt := time.Unix(response.Data.ExpireTime, 0).UTC()
		if remoteExpiresAt.After(now) {
			expiresAt = remoteExpiresAt
		}
	}

	b.mu.Lock()
	if old := b.sessions[token]; old != nil {
		old.cancel()
	}
	b.sessions[token] = &douyinBrowserRuntime{
		ctx:       tabCtx,
		cancel:    cancel,
		expiresAt: expiresAt,
	}
	b.mu.Unlock()

	return douyinBrowserCreateResult{
		Token:     token,
		QRCodeURL: qrcodeURL,
		ExpiresAt: expiresAt,
	}, nil
}

func (b *chromedpDouyinBrowser) Poll(ctx context.Context, token string) (douyinBrowserPollResult, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return douyinBrowserPollResult{}, ErrLoginSessionNotFound
	}
	session := b.session(token)
	if session == nil {
		return douyinBrowserPollResult{}, ErrLoginSessionNotFound
	}
	if !session.expiresAt.IsZero() && time.Now().UTC().After(session.expiresAt) {
		return douyinBrowserPollResult{State: StateExpired}, nil
	}

	runCtx, cancelRun := context.WithTimeout(session.ctx, 10*time.Second)
	defer cancelRun()

	var body string
	if err := chromedp.Run(runCtx, latestDouyinCapturedBody(douyinBrowserPollMatch, &body)); err != nil {
		return douyinBrowserPollResult{}, err
	}
	if strings.TrimSpace(body) == "" {
		return douyinBrowserPollResult{State: StatePendingScan}, nil
	}

	var response struct {
		Message string `json:"message"`
		Data    struct {
			ErrorCode   int             `json:"error_code"`
			Description string          `json:"description"`
			Status      json.RawMessage `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return douyinBrowserPollResult{}, fmt.Errorf("decode douyin qrcode poll response: %w", err)
	}
	if response.Data.ErrorCode != 0 {
		return douyinBrowserPollResult{}, fmt.Errorf("douyin qrcode poll failed: %s", firstNonEmpty(response.Data.Description, response.Message, "invalid response"))
	}

	switch douyinStatus(response.Data.Status) {
	case "1", "new":
		return douyinBrowserPollResult{State: StatePendingScan}, nil
	case "2", "scanned":
		return douyinBrowserPollResult{State: StatePendingConfirm}, nil
	case "3", "confirmed", "success", "succeeded":
		cookies, err := b.waitLoginCookies(session.ctx, 3*time.Second)
		if err != nil {
			return douyinBrowserPollResult{}, err
		}
		return douyinBrowserPollResult{
			State:   StateSucceeded,
			Cookie:  cookieHeader(cookies),
			Cookies: cookies,
		}, nil
	case "4", "5", "expired", "canceled", "cancelled":
		return douyinBrowserPollResult{State: StateExpired}, nil
	default:
		return douyinBrowserPollResult{}, fmt.Errorf("douyin qrcode poll status %s", string(response.Data.Status))
	}
}

func (b *chromedpDouyinBrowser) Close(token string) {
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

func (b *chromedpDouyinBrowser) session(token string) *douyinBrowserRuntime {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sessions[token]
}

func (b *chromedpDouyinBrowser) cookies(ctx context.Context) (map[string]string, error) {
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

func (b *chromedpDouyinBrowser) waitLoginCookies(ctx context.Context, timeout time.Duration) (map[string]string, error) {
	deadline := time.Now().Add(timeout)
	var last map[string]string
	for {
		cookies, err := b.cookies(ctx)
		if err != nil {
			return nil, err
		}
		last = cookies
		if douyinHasLoginCookie(cookies) || time.Now().After(deadline) {
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
	)
	if strings.TrimSpace(browserPath) != "" {
		allocatorOptions = append(allocatorOptions, chromedp.ExecPath(strings.TrimSpace(browserPath)))
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

func waitDouyinCapturedBody(pattern string, body *string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		for {
			if err := latestDouyinCapturedBody(pattern, body).Do(ctx); err != nil {
				return err
			}
			if strings.TrimSpace(*body) != "" {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
	})
}

func latestDouyinCapturedBody(pattern string, body *string) chromedp.Action {
	expression := fmt.Sprintf(`(() => {
		const pattern = %q;
		const captures = window.__rayleaDouyinQRCaptures || [];
		for (let i = captures.length - 1; i >= 0; i--) {
			const item = captures[i];
			if (item && typeof item.url === "string" && item.url.includes(pattern)) {
				return item.body || "";
			}
		}
		return "";
	})()`, pattern)
	return chromedp.Evaluate(expression, body)
}

func douyinStatus(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(strings.ToLower(text))
	}
	var number int
	if err := json.Unmarshal(raw, &number); err == nil {
		return fmt.Sprintf("%d", number)
	}
	return strings.Trim(strings.ToLower(value), `"`)
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

const douyinBrowserCaptureScript = `
(() => {
  if (window.__rayleaDouyinQRHookInstalled) return;
  window.__rayleaDouyinQRHookInstalled = true;
  window.__rayleaDouyinQRCaptures = [];
  const capture = (url, status, body) => {
    try {
      const item = { url: String(url || ""), status: Number(status || 0), body: String(body || ""), ts: Date.now() };
      if (item.url.includes("/passport/web/get_qrcode/") || item.url.includes("/passport/web/check_qrconnect/")) {
        window.__rayleaDouyinQRCaptures.push(item);
        if (window.__rayleaDouyinQRCaptures.length > 50) {
          window.__rayleaDouyinQRCaptures.shift();
        }
      }
    } catch (_) {}
  };
  const originalFetch = window.fetch;
  if (originalFetch) {
    window.fetch = function(input, init) {
      const requestedURL = typeof input === "string" ? input : (input && input.url) || "";
      return originalFetch.apply(this, arguments).then((response) => {
        try {
          response.clone().text().then((body) => capture(requestedURL || response.url, response.status, body)).catch(() => {});
        } catch (_) {}
        return response;
      });
    };
  }
  const originalOpen = XMLHttpRequest.prototype.open;
  const originalSend = XMLHttpRequest.prototype.send;
  XMLHttpRequest.prototype.open = function(method, url) {
    this.__rayleaDouyinQRURL = url;
    return originalOpen.apply(this, arguments);
  };
  XMLHttpRequest.prototype.send = function() {
    this.addEventListener("load", function() {
      try {
        capture(this.responseURL || this.__rayleaDouyinQRURL, this.status, this.responseText);
      } catch (_) {}
    });
    return originalSend.apply(this, arguments);
  };
})();
`
