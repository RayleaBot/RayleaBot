package thirdpartyapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

type thirdPartyMediaRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn thirdPartyMediaRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestThirdPartyMediaStreamsAllowedBilibiliImage(t *testing.T) {
	t.Parallel()

	handler := NewThirdPartyHandlers(nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://i0.hdslb.com/bfs/face/up.jpg" {
			t.Fatalf("unexpected media url: %s", request.URL.String())
		}
		if request.Header.Get("Referer") == "" || request.Header.Get("User-Agent") == "" {
			t.Fatalf("expected Bilibili image request headers, got %#v", request.Header)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpeg; charset=binary"}},
			Body:       io.NopCloser(strings.NewReader("jpeg-bytes")),
			Request:    request,
		}, nil
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/media?url=https%3A%2F%2Fi0.hdslb.com%2Fbfs%2Fface%2Fup.jpg", nil)
	recorder := httptest.NewRecorder()

	handler.HandleThirdPartyMedia().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("media status = %d, want 200 (%s)", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", got)
	}
	if recorder.Body.String() != "jpeg-bytes" {
		t.Fatalf("unexpected media body: %q", recorder.Body.String())
	}
}

func TestThirdPartyMediaRejectsUnsupportedURL(t *testing.T) {
	t.Parallel()

	handler := NewThirdPartyHandlers(nil, nil, nil, thirdPartyMediaRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected upstream request: %s", request.URL.String())
		return nil, nil
	}))
	for _, rawURL := range []string{
		"http%3A%2F%2Fi0.hdslb.com%2Fbfs%2Fface%2Fup.jpg",
		"https%3A%2F%2Fexample.com%2Fbfs%2Fface%2Fup.jpg",
		"https%3A%2F%2Fi0.hdslb.com%2Fnot-bfs%2Fup.jpg",
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/third-party/media?url="+rawURL, nil)
		recorder := httptest.NewRecorder()

		handler.HandleThirdPartyMedia().ServeHTTP(recorder, request)

		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("media status for %s = %d, want 400", rawURL, recorder.Code)
		}
		var envelope httpapi.ErrorEnvelope
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatalf("decode error envelope: %v", err)
		}
		if envelope.Error.Code != codeInvalidRequest {
			t.Fatalf("error code = %q, want %q", envelope.Error.Code, codeInvalidRequest)
		}
	}
}
