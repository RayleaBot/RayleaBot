package douyin

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const douyinBrowserResolveTimeout = 25 * time.Second
const douyinBrowserPageTimeout = 12 * time.Second
const douyinBrowserFetchTimeout = 5 * time.Second

func (b *ChromedpBrowser) ResolveUser(ctx context.Context, query string, cookieSets []map[string]string) ([]thirdparty.AccountProfile, bool, error) {
	if b == nil {
		return nil, false, nil
	}
	normalizedQuery := strings.TrimSpace(query)
	if normalizedQuery == "" {
		return nil, false, nil
	}
	var firstErr error
	for _, cookies := range douyinResolveCookieAttempts(cookieSets) {
		profiles, err := b.searchUsers(ctx, normalizedQuery, cookies)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(profiles) > 0 {
			return profiles, exactProfileMatch(profiles, normalizedQuery), nil
		}
	}
	if firstErr != nil {
		return nil, false, firstErr
	}
	return nil, false, nil
}

func (b *ChromedpBrowser) searchUsers(ctx context.Context, query string, cookies map[string]string) ([]thirdparty.AccountProfile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancelRun := context.WithTimeout(ctx, douyinBrowserResolveTimeout)
	defer cancelRun()
	tabCtx, cancelBrowser := newDouyinBrowserContext(b.browserPath, b.browserArgs)
	defer cancelBrowser()

	searchPage := "https://www.douyin.com/search/" + url.PathEscape(strings.TrimSpace(query)) + "?type=user"
	if err := runDouyinBrowserActions(tabCtx, douyinBrowserPageTimeout,
		network.Enable(),
		emulation.SetTimezoneOverride("Asia/Shanghai"),
		emulation.SetFocusEmulationEnabled(true),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return setDouyinBrowserCookies(ctx, cookies)
		}),
		chromedp.Navigate(douyinServiceURL),
		chromedp.WaitReady("body"),
		chromedp.Sleep(2*time.Second),
		chromedp.Navigate(searchPage),
		chromedp.WaitReady("body"),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return nil, err
	}
	if err := runCtx.Err(); err != nil {
		return nil, err
	}

	var firstErr error
	for _, requestPath := range douyinBrowserSearchPathsFor(query, cookies) {
		var raw string
		if err := runCtx.Err(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			break
		}
		if err := runDouyinBrowserActions(tabCtx, douyinBrowserFetchTimeout, evaluateDouyinBrowserSearch(requestPath, &raw)); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		profiles, err := douyinSearchProfilesFromJSON(raw, query)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if len(profiles) > 0 {
			return profiles, nil
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, nil
}

func setDouyinBrowserCookies(ctx context.Context, cookies map[string]string) error {
	for name, value := range cookies {
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" || value == "" {
			continue
		}
		if err := network.SetCookie(name, value).
			WithURL(douyinServiceURL).
			WithPath("/").
			Do(ctx); err != nil {
			return err
		}
	}
	return nil
}

func douyinBrowserSearchPathsFor(query string, cookies map[string]string) []string {
	rawURLs := douyinSearchURLsFor(query, cookies)
	paths := make([]string, 0, len(rawURLs))
	seen := map[string]bool{}
	for _, rawURL := range rawURLs {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		path := parsed.RequestURI()
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	return paths
}

func evaluateDouyinBrowserSearch(requestPath string, raw *string) chromedp.Action {
	js := douyinBrowserSearchScript(requestPath)
	return chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.Evaluate(js, raw, func(params *runtime.EvaluateParams) *runtime.EvaluateParams {
			return params.WithAwaitPromise(true)
		}).Do(ctx)
	})
}

func douyinBrowserSearchScript(requestPath string) string {
	encodedPath, _ := json.Marshal(requestPath)
	return `(function(){
var u = ` + string(encodedPath) + `;
u += (u.indexOf('?') === -1 ? '?' : '&') + 't=' + Date.now();
var headers = {'Accept': 'application/json, text/plain, */*'};
try {
var signer = window.byted_acrawler && window.byted_acrawler.frontierSign;
if (typeof signer === 'function') {
var signed = signer({url: u});
if (signed && typeof signed === 'object') {
if (signed['X-Bogus']) headers['X-Bogus'] = signed['X-Bogus'];
if (signed.url) u = signed.url;
if (signed.signed_url) u = signed.signed_url;
}
}
} catch(e) {}
return fetch(u, {
credentials: 'include',
headers: headers
}).then(function(r){ return r.text(); }).catch(function(e){
return JSON.stringify({error:e && e.message ? e.message : String(e)});
});
})()`
}

func runDouyinBrowserActions(tabCtx context.Context, timeout time.Duration, actions ...chromedp.Action) error {
	actionCtx, cancel := context.WithTimeout(tabCtx, timeout)
	defer cancel()
	return chromedp.Run(actionCtx, actions...)
}
