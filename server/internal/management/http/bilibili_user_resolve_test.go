package managementhttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func TestBilibiliUserResolveExactUID(t *testing.T) {
	t.Parallel()

	handler := NewBilibiliHandlers(nil, nil, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/x/space/acc/info" || r.URL.Query().Get("mid") != "1000001" {
			t.Fatalf("unexpected request URL: %s", r.URL.String())
		}
		if !strings.Contains(r.Header.Get("Referer"), "1000001") {
			t.Fatalf("expected UID referer, got %q", r.Header.Get("Referer"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"code": 0,
				"data": {
					"mid": 1000001,
					"name": "测试 UP",
					"face": "https://i0.hdslb.com/bfs/face/test-up.jpg",
					"fans": 7000000
				}
			}`)),
			Header: make(http.Header),
		}, nil
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/bilibili/users/resolve?query=1000001", nil)
	handler.HandleBilibiliUserResolve()(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d body %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, want := range []string{`"exact":true`, `"uid":"1000001"`, `"name":"测试 UP"`, `"fans":7000000`} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %s: %s", want, body)
		}
	}
}

func TestBilibiliUserResolveSearchCandidates(t *testing.T) {
	t.Parallel()

	handler := NewBilibiliHandlers(nil, nil, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/x/web-interface/search/type" || r.URL.Query().Get("search_type") != "bili_user" || r.URL.Query().Get("keyword") != "test" {
			t.Fatalf("unexpected request URL: %s", r.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{
				"code": 0,
				"data": {
					"result": [
						{"mid": "1000002", "uname": "<em class=\"keyword\">test</em>-official", "upic": "//i0.hdslb.com/bfs/face/a.jpg", "fans": 1200}
					]
				}
			}`)),
			Header: make(http.Header),
		}, nil
	}))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/bilibili/users/resolve?query=test", nil)
	handler.HandleBilibiliUserResolve()(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d body %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, want := range []string{`"exact":false`, `"uid":"1000002"`, `"name":"test-official"`, `"avatar_url":"https://i0.hdslb.com/bfs/face/a.jpg"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %s: %s", want, body)
		}
	}
	if strings.Contains(body, "<em") {
		t.Fatalf("response leaked search markup: %s", body)
	}
}
