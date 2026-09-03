package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	douyinBrowserResolveTimeout  = 55 * time.Second
	douyinFetchAttemptTimeout    = 15 * time.Second
	douyinAcrawlerReadyTimeout   = 15 * time.Second
	douyinAcrawlerReadyPollEvery = 500 * time.Millisecond
)

// ResolveUser 在登录 profile 浏览器内执行用户搜索。抖音风控把登录会话与
// 端侧设备指纹绑定，纯 HTTP 请求即使携带完整 CK 也会被 BDTuring 网关
// 拦截（要求滑块验证），只有浏览器内由 sec_sdk 环境发出的请求能通过。
// 因此该解析必须复用扫码登录的持久化 profile，并与登录窗口互斥；搜索
// 不需要用户交互，使用 headless 模式避免在宿主桌面弹出窗口。
func (b *ChromedpBrowser) ResolveUser(ctx context.Context, query string, cookieSets []map[string]string) ([]thirdparty.AccountProfile, bool, error) {
	if strings.TrimSpace(query) == "" {
		return nil, false, nil
	}
	if b.options.UserDataDir == "" {
		return nil, false, nil
	}
	start := time.Now()
	logStage := func(stage string, fields ...any) {
		if b.options.Logger != nil {
			fields = append(fields, "stage_elapsed_ms", time.Since(start).Milliseconds())
			b.options.Logger.Info("抖音浏览器解析用户“"+query+"”："+stage, append([]any{"component", "douyin_resolve", "query", query}, fields...)...)
		}
	}
	fail := func(stage string, err error) ([]thirdparty.AccountProfile, bool, error) {
		if b.options.Logger != nil {
			b.options.Logger.Warn("抖音浏览器解析用户“"+query+"”在 "+stage+" 阶段失败；本次没有返回候选用户。原因："+err.Error(), "component", "douyin_resolve", "query", query, "stage", stage, "stage_elapsed_ms", time.Since(start).Milliseconds(), "err", err.Error(),
				"browser_log", filepath.Join(os.TempDir(), "rayleabot-douyin-browser.log"))
		}
		return nil, false, fmt.Errorf("douyin browser search: %w", err)
	}
	if b.profileSlot != nil {
		select {
		case <-b.profileSlot:
			defer b.releaseProfileSlot()
		default:
			return nil, false, fmt.Errorf("douyin browser search: %w: login profile is busy", thirdparty.ErrQRLoginBrowserBusy)
		}
	}
	path := resolveDouyinBrowserPath(b.options.ConfiguredBrowserPath, b.options.ManagedBrowserPath)
	logStage("启动浏览器", "browser_path", path, "cookie_sets", len(cookieSets))
	tabCtx, cancelBrowser, err := newDouyinBrowserContext(ctx, browserLaunchAttempt{mode: BrowserModeHeadless, browserPath: path, useProfile: true}, b.options.BrowserArgs, b.options.UserDataDir)
	if err != nil {
		return fail("launch", err)
	}
	defer cancelBrowser()
	logStage("浏览器已就绪")
	tabCtx, cancelTimeout := context.WithTimeout(tabCtx, douyinBrowserResolveTimeout)
	defer cancelTimeout()
	if err := overrideDouyinBrowserUserAgent(tabCtx); err != nil {
		return fail("user_agent", err)
	}
	if len(cookieSets) > 0 {
		seedCtx, cancelSeed := douyinBrowserActionContext(tabCtx, ctx, 8*time.Second)
		seedErr := seedDouyinBrowserCookies(seedCtx, cookieSets)
		cancelSeed()
		if seedErr != nil {
			return fail("seed_cookies", seedErr)
		}
		logStage("注入账号 Cookie", "cookie_count", len(cookieSets))
	}
	searchPage := "https://www.douyin.com/search/" + url.QueryEscape(strings.TrimSpace(query)) + "?type=user"
	if err := chromedp.Run(tabCtx,
		network.Enable(),
		emulation.SetTimezoneOverride("Asia/Shanghai"),
		emulation.SetFocusEmulationEnabled(true),
		chromedp.Navigate(searchPage),
		chromedp.WaitReady("body"),
		waitForDouyinAcrawler(),
	); err != nil {
		return fail("navigate", err)
	}
	logStage("搜索页与签名库就绪")
	if err := ctx.Err(); err != nil {
		return fail("caller_ctx", err)
	}

	body, err := b.fetchSearchDocument(tabCtx, query)
	if err != nil {
		return fail("fetch", err)
	}
	logStage("fetch 完成", "bytes", len(body))
	profiles, err := douyinSearchProfilesFromJSON(body, query)
	if err != nil {
		return fail("parse", err)
	}
	logStage("解析完成", "profiles", len(profiles))
	if len(profiles) > 0 {
		return profiles, exactProfileMatch(profiles, query), nil
	}
	return nil, false, nil
}

// seedDouyinBrowserCookies 把账号 CK 注入浏览器 cookie jar。持久化 profile
// 里的会话可能因浏览器被强杀而丢失（Chromium 非正常退出不落盘 cookie），
// 用账号存储里的 CK 兜底恢复登录态后再导航搜索页。
func seedDouyinBrowserCookies(ctx context.Context, cookieSets []map[string]string) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		for _, cookies := range cookieSets {
			for name, value := range cookies {
				name = strings.TrimSpace(name)
				if name == "" || strings.TrimSpace(value) == "" {
					continue
				}
				if err := network.SetCookie(name, value).WithDomain(".douyin.com").WithPath("/").Do(ctx); err != nil {
					return err
				}
			}
		}
		return nil
	}))
}

// waitForDouyinAcrawler 等待页面加载字节签名库（byted_acrawler）。
// 签名函数必须在页面脚本执行完成后才可用，fetch 前轮询等待。
func waitForDouyinAcrawler() chromedp.Action {
	probe := `(function(){
		if (window.byted_acrawler && typeof window.byted_acrawler.frontierSign === 'function') { return true; }
		return false;
	})()`
	deadline := time.Now().Add(douyinAcrawlerReadyTimeout)
	return chromedp.ActionFunc(func(ctx context.Context) error {
		for {
			if time.Now().After(deadline) {
				return fmt.Errorf("douyin browser search: byted_acrawler did not become ready")
			}
			var ready bool
			if err := chromedp.Evaluate(probe, &ready).Do(ctx); err != nil {
				return err
			}
			if ready {
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(douyinAcrawlerReadyPollEvery):
			}
		}
	})
}

// fetchSearchDocument 在页面上下文构造 discover/search 请求并 fetch。
// 风控时间窗内该请求可能被平台挂起（pending 直到超时），单次尝试限时
// douyinFetchAttemptTimeout；挂起或失败时重新导航搜索页刷新会话后再试一次。
// 环境相关参数（屏幕、CPU、内存、UA 版本）从真实浏览器读取，版本字段
// 对齐真实抖音 web 端形态；签名由页面内 byted_acrawler.frontierSign
// 完成，请求自动携带 sec_sdk 注入的 uifid 等环境头。
func (b *ChromedpBrowser) fetchSearchDocument(ctx context.Context, query string) (string, error) {
	var body string
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			searchPage := "https://www.douyin.com/search/" + url.QueryEscape(strings.TrimSpace(query)) + "?type=user"
			if err := chromedp.Run(ctx,
				chromedp.Navigate(searchPage),
				chromedp.WaitReady("body"),
				waitForDouyinAcrawler(),
			); err != nil {
				return "", fmt.Errorf("douyin browser search: %w", err)
			}
		}
		fetchCtx, cancel := context.WithTimeout(ctx, douyinFetchAttemptTimeout)
		body, lastErr = b.fetchSearchDocumentOnce(fetchCtx, query)
		cancel()
		if lastErr == nil {
			return body, nil
		}
		if ctx.Err() != nil {
			return "", fmt.Errorf("douyin browser search: %w", ctx.Err())
		}
		if b.options.Logger != nil {
			b.options.Logger.Warn(fmt.Sprintf("抖音浏览器搜索请求第 %d 次失败；页面将重新导航后重试。原因：%s", attempt, lastErr.Error()), "component", "douyin_resolve", "attempt", attempt, "err", lastErr.Error())
		}
	}
	return "", fmt.Errorf("douyin browser search: %w", lastErr)
}

// fetchSearchDocumentOnce 执行单次搜索请求。脚本是 async IIFE：chromedp
// 默认不等待 Promise（结果被序列化成 {}），必须显式 WithAwaitPromise 才能
// 拿到响应文本。失败后检查页面状态写诊断日志，区分平台验证拦截与挂起。
func (b *ChromedpBrowser) fetchSearchDocumentOnce(ctx context.Context, query string) (string, error) {
	var body string
	err := chromedp.Run(ctx, chromedp.Evaluate(douyinBrowserSearchScript(query), &body, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	if err == nil {
		if b.options.Logger != nil {
			b.options.Logger.Debug(fmt.Sprintf("抖音浏览器搜索响应已读取，共 %d 字节；响应正文不会写入日志。", len(body)), "component", "douyin_resolve", "bytes", len(body))
		}
		return body, nil
	}
	b.logSearchPageState(ctx)
	return "", fmt.Errorf("douyin browser search: %w", err)
}

// logSearchPageState 在搜索请求失败后检查页面状态（验证页？签名库挂起？），
// 帮助区分平台验证拦截与页面加载问题。诊断在独立短超时上下文执行；
// Evaluate 超时说明页面主线程可能被占满，再尝试 CDP 截图留证。
func (b *ChromedpBrowser) logSearchPageState(ctx context.Context) {
	if b.options.Logger == nil {
		return
	}
	diagCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	script := `(async () => JSON.stringify({
	  title: document.title,
	  href: location.href,
	  readyState: document.readyState,
	  acrawler: !!(window.byted_acrawler && typeof window.byted_acrawler.frontierSign === 'function'),
	  verifyEl: !!document.querySelector('[id*="captcha"],[class*="captcha"],[id*="verify"],[class*="verify"]'),
	  hasBody: !!document.body
	}))()`
	var state string
	err := chromedp.Run(diagCtx, chromedp.Evaluate(script, &state, func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}))
	if err == nil {
		b.options.Logger.Warn("抖音浏览器搜索请求失败；已记录不含页面正文的浏览器状态，用于区分验证拦截和加载失败。", "component", "douyin_resolve", "state", state)
		return
	}
	b.options.Logger.Warn("抖音浏览器搜索请求失败，且页面状态诊断不可用；将尝试保存失败页面截图。原因："+err.Error(), "component", "douyin_resolve", "err", err.Error())
	shotPath := filepath.Join(os.TempDir(), "rayleabot-douyin-search-page.png")
	shotCtx, cancelShot := context.WithTimeout(ctx, 5*time.Second)
	defer cancelShot()
	shotErr := chromedp.Run(shotCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		data, err := page.CaptureScreenshot().WithCaptureBeyondViewport(false).Do(ctx)
		if err != nil {
			return err
		}
		return os.WriteFile(shotPath, data, 0o644)
	}))
	if shotErr != nil {
		b.options.Logger.Warn("抖音浏览器搜索失败后的截图诊断也不可用；本次仅保留结构化错误。原因："+shotErr.Error(), "component", "douyin_resolve", "err", shotErr.Error())
		return
	}
	b.options.Logger.Warn("抖音浏览器搜索失败页面已保存到 "+shotPath+"；可使用该截图继续诊断。", "component", "douyin_resolve", "screenshot", shotPath)
}

func douyinBrowserSearchScript(query string) string {
	encoded, _ := json.Marshal(strings.TrimSpace(query))
	return fmt.Sprintf(`(async () => {
  const keyword = %s;
  const ua = navigator.userAgent || '';
  const chromeVersion = (ua.match(/Chrome\/([\d.]+)/) || [])[1] || '';
  // 平台字段随实际运行环境动态取值，避免硬编码与 UA 信号不一致
  // 被风控识别为可疑环境。
  const osName = ua.indexOf('Windows') >= 0 ? 'Windows' : (ua.indexOf('Mac OS') >= 0 ? 'Mac OS' : 'Linux');
  const browserPlatform = ua.indexOf('Windows') >= 0 ? 'Win32' : (ua.indexOf('Mac OS') >= 0 ? 'MacIntel' : 'Linux x86_64');
  const pcLibraDivert = ua.indexOf('Windows') >= 0 ? 'Windows' : (ua.indexOf('Mac OS') >= 0 ? 'Mac' : 'Linux');
  const params = new URLSearchParams({
    device_platform: 'webapp',
    aid: '6383',
    channel: 'channel_pc_web',
    search_channel: 'aweme_user_web',
    keyword: keyword,
    search_source: 'normal_search',
    query_correct_type: '1',
    is_filter_search: '0',
    from_group_id: '',
    disable_rs: '0',
    offset: '0',
    count: '12',
    need_filter_settings: '1',
    list_type: 'single',
    pc_search_top_1_params: JSON.stringify({enable_ai_search_top_1: 1}),
    update_version_code: '170400',
    pc_client_type: '1',
    pc_libra_divert: pcLibraDivert,
    support_h265: '1',
    support_dash: '1',
    cpu_core_num: String(navigator.hardwareConcurrency || 16),
    version_code: '170400',
    version_name: '17.4.0',
    cookie_enabled: 'true',
    screen_width: String(window.screen.width || 1920),
    screen_height: String(window.screen.height || 1080),
    browser_language: navigator.language || 'zh-CN',
    browser_platform: browserPlatform,
    browser_name: 'Chrome',
    browser_version: chromeVersion,
    browser_online: 'true',
    engine_name: 'Blink',
    engine_version: chromeVersion,
    os_name: osName,
    os_version: '10',
    device_memory: String(navigator.deviceMemory || 8),
    platform: 'PC',
    downlink: '10',
    effective_type: '4g',
    round_trip_time: '100'
  });
  const path = '/aweme/v1/web/discover/search/?' + params.toString();
  let signed = {};
  if (window.byted_acrawler && typeof window.byted_acrawler.frontierSign === 'function') {
    signed = window.byted_acrawler.frontierSign({url: path, method: 'GET'}) || {};
  }
  const xBogus = signed['X-Bogus'] || signed['x-bogus'] || '';
  const url = new URL(path, location.origin);
  if (xBogus) {
    url.searchParams.set('X-Bogus', xBogus);
  }
  // 与页面真实调用一致的 XMLHttpRequest（axios 底层），并在脚本内部限时，
  // 避免平台挂起请求时 chromedp 只能等到外层上下文超时。
  const responseText = await new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('GET', url.toString(), true);
    xhr.withCredentials = true;
    if (xBogus) {
      xhr.setRequestHeader('X-Bogus', xBogus);
    }
    xhr.timeout = 13000;
    xhr.onload = () => resolve(xhr.responseText);
    xhr.onerror = () => reject(new Error('xhr network error'));
    xhr.ontimeout = () => reject(new Error('xhr timeout'));
    xhr.send();
  });
  return responseText;
})()`, encoded)
}
