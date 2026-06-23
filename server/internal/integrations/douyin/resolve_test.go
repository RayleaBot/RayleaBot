package douyin

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestDouyinSearchURLsPreferUserSearchItemEndpoint(t *testing.T) {
	t.Parallel()

	urls := douyinSearchURLsFor("洛天依", nil)
	if len(urls) == 0 {
		t.Fatal("expected search urls")
	}
	parsed, err := url.Parse(urls[0])
	if err != nil {
		t.Fatalf("parse search url: %v", err)
	}
	if parsed.Path != "/aweme/v1/web/search/item/" {
		t.Fatalf("search path = %q, want current search item endpoint", parsed.Path)
	}
	query := parsed.Query()
	if query.Get("keyword") != "洛天依" || query.Get("search_channel") != "aweme_user_web" || query.Get("type") != "user" {
		t.Fatalf("search query = %s, want user search item params", parsed.RawQuery)
	}
	if len(urls) < 2 {
		t.Fatal("expected video search item fallback")
	}
	fallback, err := url.Parse(urls[1])
	if err != nil {
		t.Fatalf("parse fallback search url: %v", err)
	}
	if fallback.Path != "/aweme/v1/web/search/item/" || fallback.Query().Get("search_channel") != "aweme_video_web" {
		t.Fatalf("fallback search url = %s, want video search item fallback", urls[1])
	}
}

func TestDouyinSearchProfilesFromSearchItemDocument(t *testing.T) {
	t.Parallel()

	profiles, err := douyinSearchProfilesFromJSON(`{
		"status_code": 0,
		"data": [{
			"card_info": {
				"data": {
					"user_list": [{
						"user_info": {
							"uid": "123456",
							"sec_uid": "MS4wLjABAAAAfixture",
							"unique_id": "luotianyi",
							"nickname": "洛天依",
							"avatar_thumb": {"url_list": ["https://p3-pc.douyinpic.com/avatar.jpg"]}
						}
					}]
				}
			}
		}]
	}`, "洛天依")
	if err != nil {
		t.Fatalf("parse search profiles: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles = %#v, want one result", profiles)
	}
	profile := profiles[0]
	if profile.UID != "luotianyi" || profile.Nickname != "洛天依" || profile.AvatarURL != "https://p3-pc.douyinpic.com/avatar.jpg" {
		t.Fatalf("profile = %#v, want search item user", profile)
	}
}

func TestDouyinSearchRetriesAPIAfterSearchPageRefreshesCookies(t *testing.T) {
	t.Parallel()

	apiCalls := 0
	client := &http.Client{Transport: douyinRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		rawURL := request.URL.String()
		switch {
		case strings.Contains(rawURL, "/aweme/v1/web/search/item/") || strings.Contains(rawURL, "/aweme/v1/web/general/search/single/"):
			apiCalls++
			if strings.Contains(request.Header.Get("Cookie"), "msToken=page-token") {
				return douyinTextResponse(request, `{"status_code":0,"data":[{"user_list":[{"user_info":{"unique_id":"luotianyi","nickname":"洛天依","avatar_medium":{"url_list":["https://p3-pc.douyinpic.com/avatar.jpg"]}}}]}]}`), nil
			}
			return douyinTextResponse(request, `{"status_code":0,"data":[]}`), nil
		case strings.Contains(rawURL, "www.douyin.com/search/"):
			response := douyinTextResponse(request, `<html><body><script id="RENDER_DATA" type="application/json">{"loaderData":{"search":{"data":[]}}}</script></body></html>`)
			response.Header.Set("Set-Cookie", "msToken=page-token; Path=/; Domain=.douyin.com")
			return response, nil
		default:
			t.Fatalf("unexpected request: %s", rawURL)
			return nil, nil
		}
	})}
	cookies := map[string]string{"sessionid": "fixture"}

	profiles, err := searchDouyinUsers(context.Background(), client, "洛天依", cookies)
	if err != nil {
		t.Fatalf("search douyin users: %v", err)
	}
	if len(profiles) != 1 || profiles[0].UID != "luotianyi" || profiles[0].Nickname != "洛天依" {
		t.Fatalf("profiles = %#v, want retried API user", profiles)
	}
	if cookies["msToken"] != "page-token" {
		t.Fatalf("cookies[msToken] = %q, want refreshed page token", cookies["msToken"])
	}
	if apiCalls <= len(douyinSearchURLsFor("洛天依", nil)) {
		t.Fatalf("api calls = %d, want API retry after page refresh", apiCalls)
	}
}

func TestDouyinBrowserSearchScriptUsesFrontierSignHeader(t *testing.T) {
	t.Parallel()

	script := douyinBrowserSearchScript("/aweme/v1/web/search/item/?keyword=%E6%B4%9B%E5%A4%A9%E4%BE%9D")
	for _, required := range []string{"frontierSign", "X-Bogus", "credentials: 'include'"} {
		if !strings.Contains(script, required) {
			t.Fatalf("browser search script missing %q: %s", required, script)
		}
	}
}

type douyinRoundTripFunc func(*http.Request) (*http.Response, error)

func (f douyinRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func douyinTextResponse(request *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}
