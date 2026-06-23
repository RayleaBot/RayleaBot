package thirdpartyapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func TestThirdPartyUserResolveReturnsPlatformProfiles(t *testing.T) {
	t.Parallel()

	handler := NewThirdPartyHandlers(nil, nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		rawURL := request.URL.String()
		switch {
		case strings.Contains(rawURL, "m.weibo.cn/api/container/getIndex"):
			return textResponse(request, `{"data":{"cards":[{"card_group":[{"user":{"id":"7556659984","screen_name":"洛天依","avatar_hd":"https://tvax1.sinaimg.cn/avatar.jpg"}}]}]}}`), nil
		case strings.Contains(rawURL, "www.douyin.com/aweme/v1/web/user/profile/other"):
			return textResponse(request, `{"status_code":0,"user":{"unique_id":"luotianyi","nickname":"洛天依","avatar_medium":{"url_list":["https://p3-pc.douyinpic.com/avatar.jpg"]}}}`), nil
		case strings.Contains(rawURL, "www.douyin.com/aweme/v1/web/general/search/single"):
			return textResponse(request, `{"status_code":0,"data":[{"user_list":[{"user_info":{"unique_id":"luotianyi","nickname":"洛天依","avatar_medium":{"url_list":["https://p3-pc.douyinpic.com/avatar.jpg"]}}}]}]}`), nil
		case strings.Contains(rawURL, "music.163.com/api/search/get/web") && request.URL.Query().Get("type") == "100":
			return textResponse(request, `{"result":{"artists":[{"id":8325,"name":"洛天依","picUrl":"https://p1.music.126.net/avatar.jpg"}]}}`), nil
		case strings.Contains(rawURL, "music.163.com/api/search/get/web"):
			return textResponse(request, `{"result":{}}`), nil
		default:
			t.Fatalf("unexpected upstream request: %s", rawURL)
			return nil, nil
		}
	}))

	tests := []struct {
		name     string
		path     string
		platform string
		uid      string
		avatar   string
	}{
		{
			name:     "weibo",
			path:     "/api/third-party/users/resolve?platform=weibo&query=%E6%B4%9B%E5%A4%A9%E4%BE%9D",
			platform: "weibo",
			uid:      "7556659984",
			avatar:   "https://tvax1.sinaimg.cn/avatar.jpg",
		},
		{
			name:     "douyin-url",
			path:     "/api/third-party/users/resolve?platform=douyin&query=https%3A%2F%2Fwww.douyin.com%2Fuser%2FMS4wLjABAAAAfixture",
			platform: "douyin",
			uid:      "luotianyi",
			avatar:   "https://p3-pc.douyinpic.com/avatar.jpg",
		},
		{
			name:     "douyin-search",
			path:     "/api/third-party/users/resolve?platform=douyin&query=%E6%B4%9B%E5%A4%A9%E4%BE%9D",
			platform: "douyin",
			uid:      "luotianyi",
			avatar:   "https://p3-pc.douyinpic.com/avatar.jpg",
		},
		{
			name:     "netease",
			path:     "/api/third-party/users/resolve?platform=netease_music&query=%E6%B4%9B%E5%A4%A9%E4%BE%9D",
			platform: "netease_music",
			uid:      "8325",
			avatar:   "https://p1.music.126.net/avatar.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()

			handler.HandleThirdPartyUserResolve().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("resolve status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
			}
			var response thirdPartyUserResolveResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Platform != tt.platform || !response.Exact || response.User == nil {
				t.Fatalf("unexpected resolve response: %#v", response)
			}
			if response.User.UID != tt.uid || response.User.Name != "洛天依" || response.User.AvatarURL != tt.avatar {
				t.Fatalf("unexpected user: %#v", response.User)
			}
		})
	}
}

func TestThirdPartyUserResolveUsesSavedPlatformCookie(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform string
		path     string
		cookie   string
		matchURL string
		body     string
		uid      string
		avatar   string
	}{
		{
			name:     "weibo",
			platform: thirdparty.PlatformWeibo,
			path:     "/api/third-party/users/resolve?platform=weibo&query=%E6%88%91%E7%9A%84%E4%B8%96%E7%95%8C",
			cookie:   "SUB=fixture;",
			matchURL: "m.weibo.cn/api/container/getIndex",
			body:     `{"data":{"cards":[{"card_group":[{"user":{"id":"7556659984","screen_name":"我的世界","avatar_hd":"https://tvax1.sinaimg.cn/weibo-avatar.jpg"}}]}]}}`,
			uid:      "7556659984",
			avatar:   "https://tvax1.sinaimg.cn/weibo-avatar.jpg",
		},
		{
			name:     "douyin",
			platform: thirdparty.PlatformDouyin,
			path:     "/api/third-party/users/resolve?platform=douyin&query=%E6%B4%9B%E5%A4%A9%E4%BE%9D",
			cookie:   "sessionid=fixture;",
			matchURL: "www.douyin.com/aweme/v1/web/general/search/single",
			body:     `{"status_code":0,"data":[{"user_list":[{"user_info":{"unique_id":"luotianyi","nickname":"洛天依","avatar_medium":{"url_list":["https://p3-pc.douyinpic.com/douyin-avatar.jpg"]}}}]}]}`,
			uid:      "luotianyi",
			avatar:   "https://p3-pc.douyinpic.com/douyin-avatar.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accounts := &stubThirdPartyUserResolveAccounts{
				accounts: []thirdparty.Account{{
					Platform:  tt.platform,
					AccountID: "primary",
					Enabled:   true,
					Credential: thirdparty.CredentialStatus{
						State: thirdparty.CredentialValid,
					},
				}},
				cookies: map[string]string{
					"primary": tt.cookie,
				},
			}
			handler := NewThirdPartyHandlers(accounts, nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				rawURL := request.URL.String()
				if strings.Contains(rawURL, tt.matchURL) {
					if !strings.Contains(request.Header.Get("Cookie"), strings.TrimSuffix(tt.cookie, ";")) {
						t.Fatalf("resolve request cookie = %q, want %q", request.Header.Get("Cookie"), tt.cookie)
					}
					return textResponse(request, tt.body), nil
				}
				t.Fatalf("unexpected upstream request: %s", rawURL)
				return nil, nil
			}))
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()

			handler.HandleThirdPartyUserResolve().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("resolve status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
			}
			var response thirdPartyUserResolveResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Platform != tt.platform || !response.Exact || response.User == nil {
				t.Fatalf("unexpected resolve response: %#v", response)
			}
			if response.User.UID != tt.uid || response.User.AvatarURL != tt.avatar {
				t.Fatalf("unexpected user: %#v", response.User)
			}
		})
	}
}

func TestThirdPartyUserResolveWeiboFallbackSearchCard(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyUserResolveAccounts{
		accounts: []thirdparty.Account{{
			Platform:  thirdparty.PlatformWeibo,
			AccountID: "primary",
			Enabled:   true,
			Credential: thirdparty.CredentialStatus{
				State: thirdparty.CredentialValid,
			},
		}},
		cookies: map[string]string{
			"primary": "SUB=fixture;",
		},
	}
	handler := NewThirdPartyHandlers(accounts, nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if !strings.Contains(request.Header.Get("Cookie"), "SUB=fixture") {
			t.Fatalf("resolve request cookie = %q, want saved weibo cookie", request.Header.Get("Cookie"))
		}
		if strings.Contains(request.URL.String(), "s.weibo.com/user") {
			return textResponse(request, `<html><body><div class="card-user"><a href="//weibo.com/u/1111111111" suda-data="key=tblog_search_weibo&value=seqid:fixture|type:3|t:0|pos:1-0|q:%E6%B4%9B%E5%A4%A9%E4%BE%9D|ext:mpos:1,click:user_name">%E6%B4%9B%E5%A4%A9%E4%BE%9D|ext:mpos:1,click:user_name</a></div><div class="card-user"><a href="//weibo.com/u/7556659984" nick-name="洛天依"><img src="//tvax1.sinaimg.cn/fallback-avatar.jpg" alt="洛天依"></a></div></body></html>`), nil
		}
		if !strings.Contains(request.URL.String(), "m.weibo.cn/api/container/getIndex") {
			t.Fatalf("unexpected upstream request: %s", request.URL.String())
		}
		containerID := request.URL.Query().Get("containerid")
		switch {
		case strings.Contains(containerID, "type=3"):
			return textResponse(request, `{"data":{"cards":[]}}`), nil
		case strings.Contains(containerID, "type=1"):
			return textResponse(request, `{"data":{"cards":[]}}`), nil
		default:
			t.Fatalf("unexpected weibo containerid: %s", containerID)
			return nil, nil
		}
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/users/resolve?platform=weibo&query=%E6%B4%9B%E5%A4%A9%E4%BE%9D", nil)
	recorder := httptest.NewRecorder()

	handler.HandleThirdPartyUserResolve().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	var response thirdPartyUserResolveResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Exact || response.User == nil {
		t.Fatalf("unexpected resolve response: %#v", response)
	}
	if response.User.UID != "7556659984" || response.User.Name != "洛天依" || response.User.AvatarURL != "https://tvax1.sinaimg.cn/fallback-avatar.jpg" {
		t.Fatalf("unexpected user: %#v", response.User)
	}
}

func TestThirdPartyUserResolveWeiboFiltersSearchTabsAndFillsAvatar(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyUserResolveAccounts{
		accounts: []thirdparty.Account{{
			Platform:  thirdparty.PlatformWeibo,
			AccountID: "primary",
			Enabled:   true,
			Credential: thirdparty.CredentialStatus{
				State: thirdparty.CredentialValid,
			},
		}},
		cookies: map[string]string{
			"primary": "SUB=fixture;",
		},
	}
	handler := NewThirdPartyHandlers(accounts, nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if !strings.Contains(request.Header.Get("Cookie"), "SUB=fixture") {
			t.Fatalf("resolve request cookie = %q, want saved weibo cookie", request.Header.Get("Cookie"))
		}
		if !strings.Contains(request.URL.String(), "m.weibo.cn/api/container/getIndex") {
			t.Fatalf("unexpected upstream request: %s", request.URL.String())
		}
		containerID := request.URL.Query().Get("containerid")
		switch {
		case strings.Contains(containerID, "100103type=3"):
			return textResponse(request, `{"data":{"cards":[{"card_group":[{"id":1,"name":"综合"},{"id":63,"title":"图片"},{"id":64,"desc1":"视频"},{"user":{"id":"5146173015","screen_name":"Vsinger_洛天依"}}]}]}}`), nil
		case strings.Contains(containerID, "1005055146173015"):
			return textResponse(request, `{"data":{"userInfo":{"id":"5146173015","screen_name":"Vsinger_洛天依","avatar_hd":"//tvax1.sinaimg.cn/vsinger-avatar.jpg"}}}`), nil
		default:
			t.Fatalf("unexpected weibo containerid: %s", containerID)
			return nil, nil
		}
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/users/resolve?platform=weibo&query=Vsinger_%E6%B4%9B%E5%A4%A9%E4%BE%9D", nil)
	recorder := httptest.NewRecorder()

	handler.HandleThirdPartyUserResolve().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	var response thirdPartyUserResolveResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Exact || response.User == nil || len(response.Candidates) != 1 {
		t.Fatalf("unexpected resolve response: %#v", response)
	}
	if response.User.UID != "5146173015" || response.User.Name != "Vsinger_洛天依" || response.User.AvatarURL != "https://tvax1.sinaimg.cn/vsinger-avatar.jpg" {
		t.Fatalf("unexpected user: %#v", response.User)
	}
}

func TestThirdPartyUserResolveDouyinSearchPageFallback(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyUserResolveAccounts{
		accounts: []thirdparty.Account{{
			Platform:  thirdparty.PlatformDouyin,
			AccountID: "primary",
			Enabled:   true,
			Credential: thirdparty.CredentialStatus{
				State: thirdparty.CredentialValid,
			},
		}},
		cookies: map[string]string{
			"primary": "sessionid=fixture;",
		},
	}
	handler := NewThirdPartyHandlers(accounts, nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		rawURL := request.URL.String()
		if !strings.Contains(request.Header.Get("Cookie"), "sessionid=fixture") {
			t.Fatalf("resolve request cookie = %q, want saved douyin cookie", request.Header.Get("Cookie"))
		}
		switch {
		case strings.Contains(rawURL, "www.douyin.com/aweme/v1/web/general/search/single"):
			return textResponse(request, `{"status_code":0,"data":[]}`), nil
		case strings.Contains(rawURL, "www.douyin.com/search/"):
			return textResponse(request, `<html><body><script id="RENDER_DATA" type="application/json">{"loaderData":{"self":{"user":{"unique_id":"ck_user","nickname":"CK用户","avatar_medium":{"url_list":["https://p3-pc.douyinpic.com/ck-avatar.jpg"]}}},"search":{"data":[{"author":{"unique_id":"luotianyi","nickname":"洛天依","avatar_medium":{"url_list":["https://p3-pc.douyinpic.com/page-avatar.jpg"]}}}]}}}</script></body></html>`), nil
		default:
			t.Fatalf("unexpected upstream request: %s", rawURL)
			return nil, nil
		}
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/users/resolve?platform=douyin&query=%E6%B4%9B%E5%A4%A9%E4%BE%9D", nil)
	recorder := httptest.NewRecorder()

	handler.HandleThirdPartyUserResolve().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	var response thirdPartyUserResolveResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Exact || response.User == nil {
		t.Fatalf("unexpected resolve response: %#v", response)
	}
	if response.User.UID != "luotianyi" || response.User.Name != "洛天依" || response.User.AvatarURL != "https://p3-pc.douyinpic.com/page-avatar.jpg" {
		t.Fatalf("unexpected user: %#v", response.User)
	}
}

func TestThirdPartyUserResolveDouyinUsesBrowserResolverAfterEmptyHTTPResults(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyUserResolveAccounts{
		accounts: []thirdparty.Account{{
			Platform:  thirdparty.PlatformDouyin,
			AccountID: "primary",
			Enabled:   true,
			Credential: thirdparty.CredentialStatus{
				State: thirdparty.CredentialValid,
			},
		}},
		cookies: map[string]string{
			"primary": "sessionid=fixture;",
		},
	}
	resolver := &stubDouyinUserResolver{
		profiles: []thirdparty.AccountProfile{{
			UID:       "luotianyi",
			Nickname:  "洛天依",
			AvatarURL: "https://p3-pc.douyinpic.com/browser-avatar.jpg",
		}},
		exact: true,
	}
	handler := NewThirdPartyHandlers(accounts, nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		rawURL := request.URL.String()
		if !strings.Contains(request.Header.Get("Cookie"), "sessionid=fixture") {
			t.Fatalf("resolve request cookie = %q, want saved douyin cookie", request.Header.Get("Cookie"))
		}
		switch {
		case strings.Contains(rawURL, "www.douyin.com/aweme/v1/web/general/search/single"):
			return textResponse(request, `{"status_code":0,"data":[]}`), nil
		case strings.Contains(rawURL, "www.douyin.com/search/"):
			return textResponse(request, `<html><body><div id="root"></div><script src="/search.js"></script></body></html>`), nil
		default:
			t.Fatalf("unexpected upstream request: %s", rawURL)
			return nil, nil
		}
	}), WithDouyinUserResolver(resolver))
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/users/resolve?platform=douyin&query=%E6%B4%9B%E5%A4%A9%E4%BE%9D", nil)
	recorder := httptest.NewRecorder()

	handler.HandleThirdPartyUserResolve().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	var response thirdPartyUserResolveResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Exact || response.User == nil {
		t.Fatalf("unexpected resolve response: %#v", response)
	}
	if response.User.UID != "luotianyi" || response.User.AvatarURL != "https://p3-pc.douyinpic.com/browser-avatar.jpg" {
		t.Fatalf("unexpected user: %#v", response.User)
	}
	if resolver.query != "洛天依" {
		t.Fatalf("browser resolver query = %q, want 洛天依", resolver.query)
	}
	if len(resolver.cookieSets) != 1 || resolver.cookieSets[0]["sessionid"] != "fixture" {
		t.Fatalf("browser resolver cookies = %#v, want saved douyin cookie", resolver.cookieSets)
	}
}

func TestThirdPartyUserResolveDouyinParsesScopedUserObject(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyUserResolveAccounts{
		accounts: []thirdparty.Account{{
			Platform:  thirdparty.PlatformDouyin,
			AccountID: "primary",
			Enabled:   true,
			Credential: thirdparty.CredentialStatus{
				State: thirdparty.CredentialValid,
			},
		}},
		cookies: map[string]string{
			"primary": "sessionid=fixture;",
		},
	}
	handler := NewThirdPartyHandlers(accounts, nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		rawURL := request.URL.String()
		if !strings.Contains(rawURL, "www.douyin.com/aweme/v1/web/general/search/single") {
			t.Fatalf("unexpected upstream request: %s", rawURL)
		}
		if !strings.Contains(request.Header.Get("Cookie"), "sessionid=fixture") {
			t.Fatalf("resolve request cookie = %q, want saved douyin cookie", request.Header.Get("Cookie"))
		}
		return textResponse(request, `{"status_code":0,"user":{"unique_id":"ck_user","nickname":"CK用户","avatar_medium":{"url_list":["https://p3-pc.douyinpic.com/ck-avatar.jpg"]}},"data":[{"user":{"unique_id":"luotianyi","nickname":"洛天依","avatar_medium":{"url_list":["https://p3-pc.douyinpic.com/target-avatar.jpg"]}}}]}`), nil
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/users/resolve?platform=douyin&query=%E6%B4%9B%E5%A4%A9%E4%BE%9D", nil)
	recorder := httptest.NewRecorder()

	handler.HandleThirdPartyUserResolve().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	var response thirdPartyUserResolveResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Exact || response.User == nil {
		t.Fatalf("unexpected resolve response: %#v", response)
	}
	if response.User.UID != "luotianyi" || response.User.Name != "洛天依" || response.User.AvatarURL != "https://p3-pc.douyinpic.com/target-avatar.jpg" {
		t.Fatalf("unexpected user: %#v", response.User)
	}
}

func TestThirdPartyUserResolveDouyinDoesNotReturnCookieAccountFromSearch(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyUserResolveAccounts{
		accounts: []thirdparty.Account{{
			Platform:  thirdparty.PlatformDouyin,
			AccountID: "primary",
			Enabled:   true,
			Credential: thirdparty.CredentialStatus{
				State: thirdparty.CredentialValid,
			},
		}},
		cookies: map[string]string{
			"primary": "sessionid=fixture;",
		},
	}
	handler := NewThirdPartyHandlers(accounts, nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		rawURL := request.URL.String()
		if !strings.Contains(request.Header.Get("Cookie"), "sessionid=fixture") {
			t.Fatalf("resolve request cookie = %q, want saved douyin cookie", request.Header.Get("Cookie"))
		}
		switch {
		case strings.Contains(rawURL, "www.douyin.com/aweme/v1/web/general/search/single"):
			return textResponse(request, `{"status_code":0,"user":{"unique_id":"ck_user","nickname":"CK用户","avatar_medium":{"url_list":["https://p3-pc.douyinpic.com/ck-avatar.jpg"]}},"data":[]}`), nil
		case strings.Contains(rawURL, "www.douyin.com/search/"):
			return textResponse(request, `<html><body><script id="RENDER_DATA" type="application/json">{"loaderData":{"self":{"user":{"unique_id":"ck_user","nickname":"CK用户","avatar_medium":{"url_list":["https://p3-pc.douyinpic.com/ck-avatar.jpg"]}}},"search":{"data":[]}}}</script></body></html>`), nil
		default:
			t.Fatalf("unexpected upstream request: %s", rawURL)
			return nil, nil
		}
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/users/resolve?platform=douyin&query=%E6%B4%9B%E5%A4%A9%E4%BE%9D", nil)
	recorder := httptest.NewRecorder()

	handler.HandleThirdPartyUserResolve().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	var response thirdPartyUserResolveResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Exact || response.User != nil || len(response.Candidates) != 0 {
		t.Fatalf("unexpected resolve response: %#v", response)
	}
	if !strings.Contains(response.Message, "抖音") {
		t.Fatalf("unexpected not found message: %q", response.Message)
	}
}

func TestThirdPartyUserResolveDouyinSearchErrorsReturnNotFoundResult(t *testing.T) {
	t.Parallel()

	handler := NewThirdPartyHandlers(nil, nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, io.ErrUnexpectedEOF
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/users/resolve?platform=douyin&query=%E6%B4%9B%E5%A4%A9%E4%BE%9D", nil)
	recorder := httptest.NewRecorder()

	handler.HandleThirdPartyUserResolve().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("resolve status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	var response thirdPartyUserResolveResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Exact || response.User != nil || len(response.Candidates) != 0 {
		t.Fatalf("unexpected resolve response: %#v", response)
	}
	if !strings.Contains(response.Message, "抖音") {
		t.Fatalf("unexpected not found message: %q", response.Message)
	}
}

type stubThirdPartyUserResolveAccounts struct {
	accounts []thirdparty.Account
	cookies  map[string]string
}

func (s *stubThirdPartyUserResolveAccounts) List(context.Context) ([]thirdparty.Account, error) {
	return s.accounts, nil
}

func (s *stubThirdPartyUserResolveAccounts) Upsert(context.Context, thirdparty.UpsertRequest) (thirdparty.Account, error) {
	return thirdparty.Account{}, nil
}

func (s *stubThirdPartyUserResolveAccounts) Delete(context.Context, string, string) error {
	return nil
}

func (s *stubThirdPartyUserResolveAccounts) ListEnabled(_ context.Context, platform string) ([]thirdparty.Account, error) {
	enabled := make([]thirdparty.Account, 0, len(s.accounts))
	for _, account := range s.accounts {
		if account.Platform == platform && account.Enabled && account.Credential.State != thirdparty.CredentialInvalid {
			enabled = append(enabled, account)
		}
	}
	return enabled, nil
}

func (s *stubThirdPartyUserResolveAccounts) ReadCookie(_ context.Context, account thirdparty.Account) (string, error) {
	return s.cookies[account.AccountID], nil
}

type stubDouyinUserResolver struct {
	query      string
	cookieSets []map[string]string
	profiles   []thirdparty.AccountProfile
	exact      bool
	err        error
}

func (s *stubDouyinUserResolver) ResolveUser(_ context.Context, query string, cookieSets []map[string]string) ([]thirdparty.AccountProfile, bool, error) {
	s.query = query
	s.cookieSets = make([]map[string]string, 0, len(cookieSets))
	for _, cookies := range cookieSets {
		cloned := make(map[string]string, len(cookies))
		for key, value := range cookies {
			cloned[key] = value
		}
		s.cookieSets = append(s.cookieSets, cloned)
	}
	return s.profiles, s.exact, s.err
}

func textResponse(request *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}
