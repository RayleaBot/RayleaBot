package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const douyinBrowserResolveTimeout = 25 * time.Second

func (b *ChromedpBrowser) ResolveUser(ctx context.Context, query string, cookieSets []map[string]string) ([]thirdparty.AccountProfile, bool, error) {
	if b == nil {
		return nil, false, nil
	}
	tabCtx, cancelBrowser := newDouyinBrowserContext(b.browserPath, b.browserArgs)
	defer cancelBrowser()
	tabCtx, cancelTimeout := context.WithTimeout(tabCtx, douyinBrowserResolveTimeout)
	defer cancelTimeout()
	if err := chromedp.Run(tabCtx,
		network.Enable(),
		emulation.SetTimezoneOverride("Asia/Shanghai"),
		emulation.SetFocusEmulationEnabled(true),
		chromedp.Navigate(douyinServiceURL),
		chromedp.WaitReady("body"),
	); err != nil {
		return nil, false, fmt.Errorf("douyin browser search: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, false, fmt.Errorf("douyin browser search: %w", err)
	}

	for _, cookies := range cookieSets {
		searchURLs := douyinSearchURLsFor(query, cookies)
		for _, rawURL := range searchURLs {
			body, err := b.fetchSearchDocument(tabCtx, rawURL)
			if err != nil {
				continue
			}
			profiles, err := douyinSearchProfilesFromJSON(body, query)
			if err != nil {
				continue
			}
			if len(profiles) > 0 {
				return profiles, exactProfileMatch(profiles, query), nil
			}
		}
	}
	return nil, false, nil
}

func (b *ChromedpBrowser) fetchSearchDocument(ctx context.Context, rawURL string) (string, error) {
	var body string
	searchPath := douyinSearchPath(rawURL)
	if err := chromedp.Run(ctx, chromedp.Evaluate(douyinBrowserSearchScript(searchPath), &body)); err != nil {
		return "", err
	}
	return body, nil
}

func douyinSearchPath(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return rawURL
	}
	if parsed.IsAbs() {
		return parsed.RequestURI()
	}
	return rawURL
}

func douyinBrowserSearchScript(searchPath string) string {
	encoded, _ := json.Marshal(strings.TrimSpace(searchPath))
	return fmt.Sprintf(`(async () => {
  const input = %s;
  const url = new URL(input, location.origin);
  let signed = {};
  if (window.byted_acrawler && typeof window.byted_acrawler.frontierSign === 'function') {
    signed = window.byted_acrawler.frontierSign({url: url.pathname + url.search, method: 'GET'}) || {};
  }
  const xBogus = signed['X-Bogus'] || signed['x-bogus'] || '';
  if (xBogus && !url.searchParams.get('X-Bogus')) {
    url.searchParams.set('X-Bogus', xBogus);
  }
  const response = await fetch(url.toString(), {
    method: 'GET',
    credentials: 'include',
    headers: xBogus ? {'X-Bogus': xBogus} : {}
  });
  return await response.text();
})()`, encoded)
}
