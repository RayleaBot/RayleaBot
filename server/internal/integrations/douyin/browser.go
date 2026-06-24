package douyin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
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
