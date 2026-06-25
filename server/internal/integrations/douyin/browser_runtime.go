package douyin

import (
	"context"
	"strings"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
)

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
		if common.HostMatches(domain, "douyin.com", "amemv.com", "bytedance.com") {
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
