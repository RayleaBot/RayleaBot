package thirdpartylogin

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

const (
	douyinBrowserLoginURL = "https://www.douyin.com/login_page?service=https%3A%2F%2Fwww.douyin.com%2F"
)

type douyinBrowserOptions struct {
	BrowserPath string
	BrowserArgs []string
	Transport   http.RoundTripper
}

type chromedpDouyinBrowser struct {
	browserPath string
	browserArgs []string
	client      *http.Client

	mu       sync.Mutex
	sessions map[string]*douyinBrowserRuntime
}

type douyinBrowserRuntime struct {
	ctx       context.Context
	cancel    context.CancelFunc
	expiresAt time.Time
	cookies   map[string]string
}

func newChromedpDouyinBrowser(options douyinBrowserOptions) *chromedpDouyinBrowser {
	return &chromedpDouyinBrowser{
		browserPath: strings.TrimSpace(options.BrowserPath),
		browserArgs: append([]string(nil), options.BrowserArgs...),
		client:      newHTTPClient(options.Transport),
		sessions:    make(map[string]*douyinBrowserRuntime),
	}
}

func (b *chromedpDouyinBrowser) Create(ctx context.Context, now time.Time) (douyinBrowserCreateResult, error) {
	tabCtx, cancel := newDouyinBrowserContext(b.browserPath, b.browserArgs)
	runCtx, cancelRun := context.WithTimeout(tabCtx, 35*time.Second)
	defer cancelRun()

	err := chromedp.Run(runCtx,
		network.Enable(),
		chromedp.Navigate(douyinBrowserLoginURL),
		chromedp.WaitReady("body"),
	)
	if err != nil {
		cancel()
		return douyinBrowserCreateResult{}, err
	}
	cookies, err := b.cookies(tabCtx)
	if err != nil {
		cancel()
		return douyinBrowserCreateResult{}, err
	}
	result, err := createDouyinQRCode(ctx, b.client, now, cookies)
	if err != nil {
		cancel()
		return douyinBrowserCreateResult{}, err
	}

	b.mu.Lock()
	if old := b.sessions[result.Token]; old != nil {
		old.cancel()
	}
	b.sessions[result.Token] = &douyinBrowserRuntime{
		ctx:       tabCtx,
		cancel:    cancel,
		expiresAt: result.ExpiresAt,
		cookies:   cloneStringMap(cookies),
	}
	b.mu.Unlock()

	result.Cookies = cloneStringMap(cookies)
	return result, nil
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

	cookies := cloneStringMap(session.cookies)
	result, err := pollDouyinQRCode(ctx, b.client, time.Now().UTC(), token, cookies)
	if err != nil {
		return douyinBrowserPollResult{}, err
	}
	if result.State == StateSucceeded && !douyinHasLoginCookie(cookies) {
		if browserCookies, err := b.waitLoginCookies(session.ctx, 3*time.Second); err == nil {
			for key, value := range browserCookies {
				cookies[key] = value
			}
			result.Cookie = cookieHeader(cookies)
			result.Cookies = cloneStringMap(cookies)
		}
	}
	session.cookies = cloneStringMap(cookies)
	return result, nil
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
