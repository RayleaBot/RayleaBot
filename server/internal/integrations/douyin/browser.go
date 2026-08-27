package douyin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	douyinBrowserLoginURL      = "https://www.douyin.com/login_page?service=https%3A%2F%2Fwww.douyin.com%2F"
	douyinBrowserLoginHost     = "login.douyin.com"
	douyinBrowserQRCodePath    = "/passport/web/get_qrcode/"
	douyinBrowserQRConnectPath = "/passport/web/check_qrconnect/"
	douyinBrowserLaunchTimeout = 20 * time.Second
	// 登录成功后导航到主页并停留，让 mssdk/sec_sdk 完成会话初始化。
	douyinBrowserHomeURL       = "https://www.douyin.com/"
	douyinBrowserSettleDelay   = 8 * time.Second
	douyinBrowserSettleTimeout = 15 * time.Second
)

type BrowserOptions struct {
	ConfiguredBrowserPath string
	ManagedBrowserPath    string
	BrowserArgs           []string
	Mode                  string
	RemoteDebuggingURL    string
	// UserDataDir 可选：可见登录窗口的持久化浏览器 profile 目录。
	// 设置后设备指纹与 sec_sdk 环境跨登录会话保留，平台会把重复登录
	// 识别为同一设备，降低新设备风控；同一时间只允许一个会话使用。
	UserDataDir      string
	Logger           *slog.Logger
	cookieReader     func(context.Context) (map[string]string, error)
	signalReader     func(context.Context, context.Context) (douyinPageSignals, error)
	attemptRunner    func(context.Context, browserLaunchAttempt, time.Time) (BrowserCreateResult, *browserRuntime, error)
	fallbackPoller   func(context.Context, context.Context, string) (douyinBrowserPollResponse, error)
	redirectFollower func(context.Context, context.Context, string) error
}

type ChromedpBrowser struct {
	options          BrowserOptions
	cookieReader     func(context.Context) (map[string]string, error)
	signalReader     func(context.Context, context.Context) (douyinPageSignals, error)
	attemptRunner    func(context.Context, browserLaunchAttempt, time.Time) (BrowserCreateResult, *browserRuntime, error)
	fallbackPoller   func(context.Context, context.Context, string) (douyinBrowserPollResponse, error)
	redirectFollower func(context.Context, context.Context, string) error

	mu       sync.Mutex
	sessions map[string]*browserRuntime
	// profileSlot 串行化持久化 profile 的使用：同一目录同时只能被一个
	// 浏览器进程占用。容量 1 信号量，获取失败时可见模式跳过本次尝试。
	profileSlot chan struct{}
}

type browserRuntime struct {
	ctx       context.Context
	cancel    context.CancelFunc
	expiresAt time.Time
	mode      string
	capture   *douyinNetworkCapture
	pollMode  browserQRCodePollMode
	closeOnce sync.Once
	pollMu    sync.Mutex

	lifecycleMu sync.Mutex
	timer       *time.Timer
	closed      bool

	mu         sync.Mutex
	cookies    map[string]string
	blockCount int
	pollSeq    uint64
	lastState  string
	// usesProfile 标记本会话占用了持久化浏览器 profile，关闭时需要释放。
	usesProfile bool
}

func NewChromedpBrowser(options BrowserOptions) *ChromedpBrowser {
	options.ConfiguredBrowserPath = strings.TrimSpace(options.ConfiguredBrowserPath)
	options.ManagedBrowserPath = strings.TrimSpace(options.ManagedBrowserPath)
	options.BrowserArgs = append([]string(nil), options.BrowserArgs...)
	options.Mode = strings.TrimSpace(strings.ToLower(options.Mode))
	if options.Mode == "" {
		options.Mode = BrowserModeAuto
	}
	options.RemoteDebuggingURL = strings.TrimSpace(options.RemoteDebuggingURL)
	options.UserDataDir = strings.TrimSpace(options.UserDataDir)
	browser := &ChromedpBrowser{options: options, sessions: make(map[string]*browserRuntime)}
	if browser.options.UserDataDir != "" {
		browser.profileSlot = make(chan struct{}, 1)
		browser.profileSlot <- struct{}{}
	}
	browser.cookieReader = options.cookieReader
	if browser.cookieReader == nil {
		browser.cookieReader = browser.readCookies
	}
	browser.signalReader = options.signalReader
	if browser.signalReader == nil {
		browser.signalReader = readDouyinPageSignals
	}
	browser.attemptRunner = options.attemptRunner
	if browser.attemptRunner == nil {
		browser.attemptRunner = browser.createAttempt
	}
	browser.fallbackPoller = options.fallbackPoller
	if browser.fallbackPoller == nil {
		browser.fallbackPoller = pollDouyinFallbackQRCode
	}
	browser.redirectFollower = options.redirectFollower
	if browser.redirectFollower == nil {
		browser.redirectFollower = followDouyinBrowserRedirect
	}
	return browser
}

func (b *ChromedpBrowser) Create(ctx context.Context, now time.Time) (BrowserCreateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return BrowserCreateResult{}, fmt.Errorf("douyin browser: %w", err)
	}
	attempts, err := douyinBrowserLaunchAttempts(
		b.options.Mode,
		b.options.RemoteDebuggingURL,
		b.options.ConfiguredBrowserPath,
		b.options.ManagedBrowserPath,
	)
	if err != nil {
		return BrowserCreateResult{}, err
	}

	return b.createWithAttempts(ctx, now, attempts)
}

func (b *ChromedpBrowser) createWithAttempts(ctx context.Context, now time.Time, attempts []browserLaunchAttempt) (BrowserCreateResult, error) {
	var lastErr error
	for index, attempt := range attempts {
		profileHeld := false
		if attempt.useProfile && b.profileSlot != nil {
			select {
			case <-b.profileSlot:
				profileHeld = true
			default:
				b.logBrowserProfileBusy()
				// 槽忙时不回退到无 profile 的 headless：无设备信誉的登录
				// 环境会触发新设备风控，且丢失既有登录态。剩余尝试里没有
				// 其他 profile 尝试时直接拒绝本次登录。
				if !douyinAttemptsUseProfile(attempts[index+1:]) {
					return BrowserCreateResult{}, fmt.Errorf("%w: login browser profile is busy, retry later", thirdparty.ErrQRLoginBrowserBusy)
				}
				continue
			}
		}
		attemptCtx, cancelAttempt := douyinBrowserAttemptContext(ctx, len(attempts)-index)
		result, runtime, err := b.attemptRunner(attemptCtx, attempt, now)
		cancelAttempt()
		if err == nil && (runtime == nil || strings.TrimSpace(result.Token) == "") {
			err = fmt.Errorf("douyin browser attempt returned an invalid session")
		}
		if err != nil && runtime != nil {
			closeDouyinBrowserRuntime(runtime)
		}
		if err != nil {
			if profileHeld {
				b.releaseProfileSlot()
			}
			if requestErr := ctx.Err(); requestErr != nil {
				return BrowserCreateResult{}, fmt.Errorf("douyin browser: %w", requestErr)
			}
			lastErr = err
			b.logBrowserFallback(attempt.mode)
			continue
		}
		runtime.usesProfile = profileHeld
		b.mu.Lock()
		old := b.sessions[result.Token]
		b.sessions[result.Token] = runtime
		b.mu.Unlock()
		if old != nil {
			closeDouyinBrowserRuntime(old)
		}
		delay := time.Until(result.ExpiresAt)
		if delay < 0 {
			delay = 0
		}
		timer := time.AfterFunc(delay, func() {
			b.closeSession(result.Token, runtime)
		})
		runtime.lifecycleMu.Lock()
		if runtime.closed {
			timer.Stop()
		} else {
			runtime.timer = timer
		}
		runtime.lifecycleMu.Unlock()
		b.logBrowserStarted(attempt.mode)
		return result, nil
	}

	if b.options.Mode != BrowserModeAuto && lastErr != nil {
		return BrowserCreateResult{}, lastErr
	}
	if lastErr != nil {
		return BrowserCreateResult{}, fmt.Errorf("%w: no configured browser mode completed login setup", thirdparty.ErrQRLoginBrowserUnavailable)
	}
	return BrowserCreateResult{}, thirdparty.ErrQRLoginBrowserUnavailable
}

// douyinAttemptsUseProfile 判断剩余尝试列表中是否还有持久化 profile 尝试。
func douyinAttemptsUseProfile(attempts []browserLaunchAttempt) bool {
	for _, attempt := range attempts {
		if attempt.useProfile {
			return true
		}
	}
	return false
}

// releaseProfileSlot 归还持久化 profile 使用权。旧浏览器进程退出是异步的，
// 延迟归还避免下一个会话在 profile 目录锁尚未释放时启动失败；残留进程
// 由下一次启动前的 killDouyinProfileProcesses 统一清理（杀完立即启动，
// 不存在迟到 kill 误伤新进程的并发窗口）。
func (b *ChromedpBrowser) releaseProfileSlot() {
	if b.profileSlot == nil {
		return
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		b.profileSlot <- struct{}{}
	}()
}

func douyinBrowserAttemptContext(ctx context.Context, attemptsRemaining int) (context.Context, context.CancelFunc) {
	if attemptsRemaining <= 1 {
		return ctx, func() {}
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return ctx, func() {}
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		attemptCtx, cancel := context.WithCancel(ctx)
		cancel()
		return attemptCtx, func() {}
	}
	return context.WithTimeout(ctx, remaining/time.Duration(attemptsRemaining))
}

func (b *ChromedpBrowser) createAttempt(ctx context.Context, attempt browserLaunchAttempt, now time.Time) (BrowserCreateResult, *browserRuntime, error) {
	tabCtx, cancel, err := newDouyinBrowserContext(ctx, attempt, b.options.BrowserArgs, b.options.UserDataDir)
	if err != nil {
		return BrowserCreateResult{}, nil, err
	}
	if err := startDouyinBrowserContext(ctx, tabCtx, cancel); err != nil {
		cancel()
		return BrowserCreateResult{}, nil, fmt.Errorf("douyin browser start failed: %w", err)
	}
	// 启动后立即把 UA 对齐到实际浏览器版本，避免硬编码 UA 与版本信号不一致。
	setupCtx, cancelSetup := douyinBrowserActionContext(tabCtx, ctx, 8*time.Second)
	setupErr := overrideDouyinBrowserUserAgent(setupCtx)
	cancelSetup()
	if setupErr != nil {
		cancel()
		return BrowserCreateResult{}, nil, fmt.Errorf("douyin browser user agent setup failed: %w", setupErr)
	}
	// 持久化 profile 会保留上次的登录态；复用同一设备登录新账号前清掉
	// 旧登录字段（保留 ttwid/s_v_web_id 等设备字段，维持设备信誉）。
	if attempt.mode == BrowserModeVisible && strings.TrimSpace(b.options.UserDataDir) != "" {
		resetCtx, cancelReset := douyinBrowserActionContext(tabCtx, ctx, 8*time.Second)
		resetErr := clearDouyinLoginStateCookies(resetCtx)
		cancelReset()
		if resetErr != nil {
			cancel()
			return BrowserCreateResult{}, nil, fmt.Errorf("douyin browser profile reset failed: %w", resetErr)
		}
	}

	capture := newDouyinNetworkCapture(tabCtx)
	actionCtx, cancelAction := douyinBrowserActionContext(tabCtx, ctx, 12*time.Second)
	err = chromedp.Run(actionCtx,
		emulation.SetTimezoneOverride("Asia/Shanghai"),
		emulation.SetFocusEmulationEnabled(true),
		chromedp.Navigate(douyinBrowserLoginURL),
		chromedp.WaitReady("body"),
	)
	cancelAction()
	if err != nil {
		cancel()
		return BrowserCreateResult{}, nil, fmt.Errorf("douyin browser navigation failed")
	}
	result, err := waitDouyinBrowserQRCode(ctx, tabCtx, capture, now)
	if err != nil {
		cancel()
		return BrowserCreateResult{}, nil, err
	}
	cookieCtx, cancelCookies := douyinBrowserActionContext(tabCtx, ctx, 3*time.Second)
	cookies, err := b.cookieReader(cookieCtx)
	cancelCookies()
	if err != nil {
		cancel()
		return BrowserCreateResult{}, nil, fmt.Errorf("douyin browser cookie read failed")
	}

	runtime := &browserRuntime{
		ctx:       tabCtx,
		cancel:    cancel,
		expiresAt: result.ExpiresAt,
		mode:      attempt.mode,
		capture:   capture,
		pollMode:  result.pollMode,
		cookies:   cloneStringMap(cookies),
		lastState: thirdparty.QRLoginStatePendingScan,
	}
	result.Cookies = cloneStringMap(cookies)
	return result, runtime, nil
}

func startDouyinBrowserContext(requestCtx, tabCtx context.Context, cancelBrowser context.CancelFunc) error {
	// Chromedp binds the allocated browser process to the context used by the first Run.
	// Keep that Run on the session context; later actions may use short-lived child contexts.
	return startDouyinBrowserContextWith(requestCtx, tabCtx, cancelBrowser, douyinBrowserLaunchTimeout, func(runCtx context.Context) error {
		return chromedp.Run(runCtx, network.Enable())
	})
}

func startDouyinBrowserContextWith(
	requestCtx context.Context,
	tabCtx context.Context,
	cancelBrowser context.CancelFunc,
	timeout time.Duration,
	run func(context.Context) error,
) error {
	result := make(chan error, 1)
	go func() {
		result <- run(tabCtx)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-result:
		return err
	case <-requestCtx.Done():
		cancelBrowser()
		return requestCtx.Err()
	case <-timer.C:
		cancelBrowser()
		return context.DeadlineExceeded
	}
}

func (b *ChromedpBrowser) Poll(ctx context.Context, token string) (BrowserPollResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return BrowserPollResult{}, fmt.Errorf("douyin browser poll: %w", err)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return BrowserPollResult{}, fmt.Errorf("douyin browser poll: login session not found")
	}
	session := b.session(token)
	if session == nil {
		return BrowserPollResult{}, fmt.Errorf("douyin browser poll: login session not found")
	}
	session.pollMu.Lock()
	defer session.pollMu.Unlock()

	if !session.expiresAt.IsZero() && time.Now().UTC().After(session.expiresAt) {
		return b.finishState(session, thirdparty.QRLoginStateExpired, nil), nil
	}
	if err := session.ctx.Err(); err != nil {
		return b.finishState(session, thirdparty.QRLoginStateFailed, nil), nil
	}

	cookieCtx, cancelCookies := douyinBrowserActionContext(session.ctx, ctx, 3*time.Second)
	browserCookies, err := b.cookieReader(cookieCtx)
	cancelCookies()
	if err != nil {
		if requestErr := ctx.Err(); requestErr != nil {
			return BrowserPollResult{}, fmt.Errorf("douyin browser poll: %w", requestErr)
		}
		if session.ctx.Err() != nil || isDouyinBrowserContextLost(err) {
			return b.finishState(session, thirdparty.QRLoginStateFailed, nil), nil
		}
		return BrowserPollResult{}, fmt.Errorf("douyin browser cookie read failed")
	}
	cookies := b.mergeCookies(session, browserCookies)
	if HasLoginCookie(cookies) {
		return b.finishState(session, thirdparty.QRLoginStateSucceeded, cookies), nil
	}

	var observedState string
	var redirectURL string
	if session.pollMode == browserQRCodePollActiveToken {
		observed, pollErr := b.fallbackPoller(ctx, session.ctx, token)
		if pollErr != nil {
			if requestErr := ctx.Err(); requestErr != nil {
				return BrowserPollResult{}, fmt.Errorf("douyin browser poll: %w", requestErr)
			}
			if session.ctx.Err() != nil || isDouyinBrowserContextLost(pollErr) {
				return b.finishState(session, thirdparty.QRLoginStateFailed, nil), nil
			}
			if errors.Is(pollErr, errDouyinQRCodePollBlocked) {
				if blocked := b.observeActivePollResult(session, true); blocked >= 2 {
					b.logBrowserFailed(session.mode, "risk_control")
					return b.finishState(session, thirdparty.QRLoginStateFailed, nil), nil
				}
				return b.finishState(session, b.currentState(session), nil), nil
			}
			b.observeActivePollResult(session, false)
			return BrowserPollResult{}, pollErr
		}
		b.observeActivePollResult(session, false)
		observedState = observed.State
		redirectURL = observed.RedirectURL
	} else {
		var pollSequence uint64
		var pollErr error
		observedState, pollSequence, pollErr = readDouyinBrowserPollState(session.capture)
		if pollErr != nil {
			if errors.Is(pollErr, errDouyinQRCodePollBlocked) {
				blocked, fresh := b.observePollResult(session, pollSequence, true)
				if !fresh {
					return b.finishState(session, b.currentState(session), nil), nil
				}
				if blocked >= 2 {
					b.logBrowserFailed(session.mode, "risk_control")
					return b.finishState(session, thirdparty.QRLoginStateFailed, nil), nil
				}
				return b.finishState(session, b.currentState(session), nil), nil
			}
			b.observePollResult(session, pollSequence, false)
			return BrowserPollResult{}, pollErr
		}
		b.observePollResult(session, pollSequence, false)
	}

	if observedState == thirdparty.QRLoginStateSucceeded && redirectURL != "" {
		if err := b.redirectFollower(ctx, session.ctx, redirectURL); err != nil {
			if requestErr := ctx.Err(); requestErr != nil {
				return BrowserPollResult{}, fmt.Errorf("douyin browser poll: %w", requestErr)
			}
			if session.ctx.Err() != nil || isDouyinBrowserContextLost(err) {
				return b.finishState(session, thirdparty.QRLoginStateFailed, nil), nil
			}
			return BrowserPollResult{}, err
		}
	}

	signals, err := b.signalReader(ctx, session.ctx)
	if err != nil {
		if requestErr := ctx.Err(); requestErr != nil {
			return BrowserPollResult{}, fmt.Errorf("douyin browser poll: %w", requestErr)
		}
		if session.ctx.Err() != nil || isDouyinBrowserContextLost(err) {
			return b.finishState(session, thirdparty.QRLoginStateFailed, nil), nil
		}
		return BrowserPollResult{}, fmt.Errorf("douyin browser state read failed")
	}
	if observedState == thirdparty.QRLoginStateSucceeded {
		if loginCookies, waitErr := b.waitLoginCookies(ctx, session.ctx, 5*time.Second); waitErr == nil {
			cookies = b.mergeCookies(session, loginCookies)
		} else if requestErr := ctx.Err(); requestErr != nil {
			return BrowserPollResult{}, fmt.Errorf("douyin browser poll: %w", requestErr)
		} else if session.ctx.Err() != nil {
			return b.finishState(session, thirdparty.QRLoginStateFailed, nil), nil
		}
		if HasLoginCookie(cookies) {
			if settled := b.settleLoginSession(ctx, session.ctx); settled != nil {
				cookies = b.mergeCookies(session, settled)
			}
			b.logCookieGaps(cookies)
			return b.finishState(session, thirdparty.QRLoginStateSucceeded, cookies), nil
		}
		observedState = thirdparty.QRLoginStatePendingConfirm
	}
	if observedState == thirdparty.QRLoginStateExpired {
		return b.finishState(session, thirdparty.QRLoginStateExpired, nil), nil
	}

	current := b.currentState(session)
	if observedState == thirdparty.QRLoginStatePendingScan && (HasLoginMarker(cookies) || signals.HasUserLogin) {
		observedState = thirdparty.QRLoginStatePendingConfirm
	}
	next := advanceDouyinBrowserState(current, observedState)
	if next != thirdparty.QRLoginStatePendingScan && signals.VerificationRequired {
		next = thirdparty.QRLoginStateVerificationRequired
	}
	return b.finishState(session, next, nil), nil
}

func advanceDouyinBrowserState(current, observed string) string {
	current = thirdparty.NormalizeQRLoginState(current)
	observed = thirdparty.NormalizeQRLoginState(observed)
	if thirdparty.IsQRLoginTerminalState(current) {
		return current
	}
	if thirdparty.IsQRLoginTerminalState(observed) {
		return observed
	}
	rank := func(state string) int {
		switch state {
		case thirdparty.QRLoginStateVerificationRequired:
			return 2
		case thirdparty.QRLoginStatePendingConfirm:
			return 1
		default:
			return 0
		}
	}
	if rank(observed) >= rank(current) {
		if observed != "" {
			return observed
		}
	}
	if current != "" {
		return current
	}
	return thirdparty.QRLoginStatePendingScan
}

func (b *ChromedpBrowser) currentState(session *browserRuntime) string {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.lastState
}

func (b *ChromedpBrowser) finishState(session *browserRuntime, state string, cookies map[string]string) BrowserPollResult {
	state = thirdparty.NormalizeQRLoginState(state)
	if state == "" {
		state = thirdparty.QRLoginStateFailed
	}
	session.mu.Lock()
	if len(cookies) > 0 {
		session.cookies = cloneStringMap(cookies)
	}
	session.lastState = state
	storedCookies := cloneStringMap(session.cookies)
	session.mu.Unlock()
	result := BrowserPollResult{State: state, Cookies: storedCookies}
	if state == thirdparty.QRLoginStateSucceeded {
		result.Cookie = cookieHeaderFromMap(storedCookies)
	}
	return result
}

func (b *ChromedpBrowser) mergeCookies(session *browserRuntime, values map[string]string) map[string]string {
	session.mu.Lock()
	defer session.mu.Unlock()
	merged := cloneStringMap(session.cookies)
	for key, value := range values {
		merged[key] = value
	}
	session.cookies = cloneStringMap(merged)
	return merged
}

func (b *ChromedpBrowser) observePollResult(session *browserRuntime, sequence uint64, blocked bool) (int, bool) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if sequence == 0 || sequence == session.pollSeq {
		return session.blockCount, false
	}
	session.pollSeq = sequence
	if blocked {
		session.blockCount++
	} else {
		session.blockCount = 0
	}
	return session.blockCount, true
}

func (b *ChromedpBrowser) observeActivePollResult(session *browserRuntime, blocked bool) int {
	session.mu.Lock()
	defer session.mu.Unlock()
	if blocked {
		session.blockCount++
	} else {
		session.blockCount = 0
	}
	return session.blockCount
}

func isDouyinBrowserContextLost(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, chromedp.ErrInvalidContext) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, fragment := range []string{"target closed", "browser closed", "websocket: close"} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func (b *ChromedpBrowser) Close(token string) {
	b.closeSession(strings.TrimSpace(token), nil)
}

func (b *ChromedpBrowser) closeSession(token string, expected *browserRuntime) {
	if token == "" {
		return
	}
	b.mu.Lock()
	session := b.sessions[token]
	if session == nil || (expected != nil && session != expected) {
		b.mu.Unlock()
		return
	}
	delete(b.sessions, token)
	b.mu.Unlock()
	if session.usesProfile {
		b.releaseProfileSlot()
	}
	closeDouyinBrowserRuntime(session)
}

func closeDouyinBrowserRuntime(session *browserRuntime) {
	if session == nil {
		return
	}
	session.closeOnce.Do(func() {
		session.lifecycleMu.Lock()
		session.closed = true
		timer := session.timer
		session.timer = nil
		session.lifecycleMu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		if session.cancel != nil {
			session.cancel()
		}
	})
}

func (b *ChromedpBrowser) SessionContext(token string) context.Context {
	session := b.session(token)
	if session == nil {
		return nil
	}
	return session.ctx
}

func (b *ChromedpBrowser) session(token string) *browserRuntime {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sessions[token]
}

func (b *ChromedpBrowser) readCookies(ctx context.Context) (map[string]string, error) {
	var values map[string]string
	runCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
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

func (b *ChromedpBrowser) waitLoginCookies(requestCtx, sessionCtx context.Context, timeout time.Duration) (map[string]string, error) {
	deadline := time.Now().Add(timeout)
	var last map[string]string
	for {
		cookieCtx, cancel := douyinBrowserActionContext(sessionCtx, requestCtx, 3*time.Second)
		cookies, err := b.cookieReader(cookieCtx)
		cancel()
		if err != nil {
			return last, err
		}
		last = cookies
		if HasLoginCookie(cookies) || time.Now().After(deadline) {
			return last, nil
		}
		select {
		case <-requestCtx.Done():
			return last, requestCtx.Err()
		case <-sessionCtx.Done():
			return last, sessionCtx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// settleLoginSession 登录成功后导航到主页并停留片刻，让 mssdk/sec_sdk 完成
// 会话初始化（生成 msToken、webid 等设备字段）后再抓取完整 CK，避免保存
// 残缺会话；任何失败都返回 nil，回退到已抓取的 Cookie，不阻断登录。
func (b *ChromedpBrowser) settleLoginSession(requestCtx, sessionCtx context.Context) map[string]string {
	settleCtx, cancel := douyinBrowserActionContext(sessionCtx, requestCtx, douyinBrowserSettleTimeout)
	defer cancel()
	if err := chromedp.Run(settleCtx,
		chromedp.Navigate(douyinBrowserHomeURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(douyinBrowserSettleDelay),
	); err != nil {
		return nil
	}
	readCtx, cancelRead := douyinBrowserActionContext(sessionCtx, requestCtx, 3*time.Second)
	settled, err := b.cookieReader(readCtx)
	cancelRead()
	if err != nil {
		return nil
	}
	return settled
}

// logCookieGaps 记录登录 Cookie 缺失的平台设备字段，提示会话初始化不完整。
func (b *ChromedpBrowser) logCookieGaps(cookies map[string]string) {
	if b.options.Logger == nil {
		return
	}
	var missing []string
	for _, name := range []string{"s_v_web_id", "ttwid", "msToken", "webid"} {
		if strings.TrimSpace(cookies[name]) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		b.options.Logger.Warn("抖音扫码登录 Cookie 缺少设备字段", "component", "douyin_qrcode", "missing", strings.Join(missing, ","))
	}
}

func (b *ChromedpBrowser) logBrowserFallback(mode string) {
	if b.options.Logger != nil {
		b.options.Logger.Warn("抖音扫码登录浏览器模式不可用", "component", "douyin_qrcode", "mode", mode)
	}
}

func (b *ChromedpBrowser) logBrowserStarted(mode string) {
	if b.options.Logger != nil {
		b.options.Logger.Info("抖音扫码登录浏览器已启动", "component", "douyin_qrcode", "mode", mode)
	}
}

func (b *ChromedpBrowser) logBrowserProfileBusy() {
	if b.options.Logger != nil {
		b.options.Logger.Warn("抖音扫码登录浏览器 profile 正被其他登录会话使用，本次跳过可见模式", "component", "douyin_qrcode")
	}
}

func (b *ChromedpBrowser) logBrowserFailed(mode, reason string) {
	if b.options.Logger != nil {
		b.options.Logger.Warn("抖音扫码登录失败", "component", "douyin_qrcode", "mode", mode, "reason", reason)
	}
}
