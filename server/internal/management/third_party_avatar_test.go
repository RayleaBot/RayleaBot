package management

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type avatarRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn avatarRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestThirdPartyAccountAvatarReturnsControlledWeiboImage(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyAccounts{getAccount: thirdparty.Account{
		Platform:  thirdparty.PlatformWeibo,
		AccountID: "primary",
		Profile: thirdparty.AccountProfile{
			AvatarURL: "https://wx1.sinaimg.cn/orj480/avatar.jpg",
		},
	}}
	transport := avatarRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Referer") != "https://weibo.com/" {
			t.Fatalf("avatar Referer = %q, want Weibo origin", request.Header.Get("Referer"))
		}
		if request.Header.Get("Cookie") != "" {
			t.Fatal("avatar request forwarded account credentials")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(strings.NewReader("jpeg")),
			Request:    request,
		}, nil
	})
	handler := NewThirdPartyHandlers(accounts, nil, nil, WithThirdPartyAvatarTransport(transport))
	router := chi.NewRouter()
	router.Get("/api/third-party/accounts/{platform}/{account_id}/avatar", handler.HandleThirdPartyAccountAvatar())
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/accounts/weibo/primary/avatar", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != "jpeg" {
		t.Fatalf("avatar response = %d %q, want 200 jpeg", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "image/jpeg" || recorder.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("unexpected avatar response headers: %#v", recorder.Header())
	}
}

func TestThirdPartyAccountAvatarNormalizesNeteaseJPGContentType(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyAccounts{getAccount: thirdparty.Account{
		Platform:  thirdparty.PlatformNeteaseMusic,
		AccountID: "primary",
		Profile: thirdparty.AccountProfile{
			AvatarURL: "https://p4.music.126.net/fixture==/avatar.jpg",
		},
	}}
	transport := avatarRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpg"}},
			Body:       io.NopCloser(strings.NewReader("jpeg")),
			Request:    request,
		}, nil
	})
	handler := NewThirdPartyHandlers(accounts, nil, nil, WithThirdPartyAvatarTransport(transport))
	router := chi.NewRouter()
	router.Get("/api/third-party/accounts/{platform}/{account_id}/avatar", handler.HandleThirdPartyAccountAvatar())
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/accounts/netease_music/primary/avatar", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("NetEase avatar response = %d content-type %q, want 200 image/jpeg", recorder.Code, recorder.Header().Get("Content-Type"))
	}
}

func TestThirdPartyAccountAvatarReturnsNotFoundCode(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyAccounts{getErr: thirdparty.ErrAccountNotFound}
	handler := NewThirdPartyHandlers(accounts, nil, nil)
	router := chi.NewRouter()
	router.Get("/api/third-party/accounts/{platform}/{account_id}/avatar", handler.HandleThirdPartyAccountAvatar())
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/accounts/weibo/missing/avatar", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("avatar status = %d, want 404 body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope httpapi.ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode avatar error: %v", err)
	}
	if envelope.Error.Code != "platform.third_party_account_not_found" {
		t.Fatalf("avatar error code = %q, want account not found", envelope.Error.Code)
	}
}

func TestThirdPartyAccountAvatarRejectsUnsupportedStoredSource(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyAccounts{getAccount: thirdparty.Account{
		Platform:  thirdparty.PlatformWeibo,
		AccountID: "primary",
		Profile: thirdparty.AccountProfile{
			AvatarURL: "https://example.test/avatar.jpg",
		},
	}}
	transportCalled := false
	transport := avatarRoundTripFunc(func(*http.Request) (*http.Response, error) {
		transportCalled = true
		return nil, nil
	})
	handler := NewThirdPartyHandlers(accounts, nil, nil, WithThirdPartyAvatarTransport(transport))
	router := chi.NewRouter()
	router.Get("/api/third-party/accounts/{platform}/{account_id}/avatar", handler.HandleThirdPartyAccountAvatar())
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/accounts/weibo/primary/avatar", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway || transportCalled {
		t.Fatalf("avatar response = %d, transportCalled=%v, want safe 502 without upstream request", recorder.Code, transportCalled)
	}
	if strings.Contains(recorder.Body.String(), "example.test") {
		t.Fatalf("avatar error leaked stored source URL: %s", recorder.Body.String())
	}
}
