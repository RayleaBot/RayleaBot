package management

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/go-chi/chi/v5"
)

type stubThirdPartyQRCodeLogin struct {
	createResult thirdparty.QRLoginCreateResult
	createErr    error
	pollResult   thirdparty.QRLoginPollResult
	pollErr      error
	cancelErr    error
	cancelled    []string
}

func (s *stubThirdPartyQRCodeLogin) Create(context.Context, string) (thirdparty.QRLoginCreateResult, error) {
	return s.createResult, s.createErr
}

func (s *stubThirdPartyQRCodeLogin) Poll(context.Context, string, string) (thirdparty.QRLoginPollResult, error) {
	return s.pollResult, s.pollErr
}

func (s *stubThirdPartyQRCodeLogin) Cancel(_ context.Context, platform, loginID string) error {
	s.cancelled = append(s.cancelled, platform+":"+loginID)
	return s.cancelErr
}

func TestThirdPartyQRCodeLoginHandlersCreateAndPoll(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 6, 8, 8, 3, 0, 0, time.UTC)
	for _, platform := range []string{thirdparty.PlatformBilibili, thirdparty.PlatformWeibo, thirdparty.PlatformDouyin, thirdparty.PlatformNeteaseMusic} {
		t.Run(platform, func(t *testing.T) {
			t.Parallel()
			qrLogin := &stubThirdPartyQRCodeLogin{
				createResult: thirdparty.QRLoginCreateResult{
					Platform:  platform,
					LoginID:   platform + "_qr_fixture",
					QRCodeURL: "https://example.test/" + platform,
					ExpiresAt: expiresAt,
					State:     thirdparty.QRLoginStatePendingScan,
				},
				pollResult: thirdparty.QRLoginPollResult{
					Platform:  platform,
					LoginID:   platform + "_qr_fixture",
					State:     thirdparty.QRLoginStateSucceeded,
					ExpiresAt: expiresAt,
					Cookie:    "CK=fixture;",
					Account: thirdparty.AccountProfile{
						UID:       "123456",
						Nickname:  "扫码账号",
						AvatarURL: "https://example.test/avatar.jpg",
					},
					SavedAccount: &thirdparty.Account{
						Platform:   platform,
						AccountID:  "123456",
						Label:      "扫码账号",
						Enabled:    true,
						Configured: true,
						Profile: thirdparty.AccountProfile{
							UID:       "123456",
							Nickname:  "扫码账号",
							AvatarURL: "https://example.test/avatar.jpg",
						},
						Credential: thirdparty.CredentialStatus{State: thirdparty.CredentialValid, CheckedAt: &expiresAt},
						UpdatedAt:  expiresAt,
					},
				},
			}
			handler := NewThirdPartyHandlers(nil, nil, qrLogin)
			router := thirdPartyQRCodeLoginRouter(handler)

			createRecorder := httptest.NewRecorder()
			createRequest := httptest.NewRequest(http.MethodPost, "/api/third-party/accounts/"+platform+"/login/qrcode", nil)
			router.ServeHTTP(createRecorder, createRequest)
			if createRecorder.Code != http.StatusOK {
				t.Fatalf("create status = %d, want 200 body=%s", createRecorder.Code, createRecorder.Body.String())
			}
			var createResponse thirdPartyQRCodeLoginCreateResponse
			if err := json.Unmarshal(createRecorder.Body.Bytes(), &createResponse); err != nil {
				t.Fatalf("decode create response: %v", err)
			}
			if createResponse.Platform != platform || createResponse.State != thirdparty.QRLoginStatePendingScan || createResponse.QRCodeURL == "" {
				t.Fatalf("unexpected create response: %#v", createResponse)
			}

			pollRecorder := httptest.NewRecorder()
			pollRequest := httptest.NewRequest(http.MethodGet, "/api/third-party/accounts/"+platform+"/login/qrcode/"+platform+"_qr_fixture", nil)
			router.ServeHTTP(pollRecorder, pollRequest)
			if pollRecorder.Code != http.StatusOK {
				t.Fatalf("poll status = %d, want 200 body=%s", pollRecorder.Code, pollRecorder.Body.String())
			}
			var pollResponse thirdPartyQRCodeLoginPollResponse
			if err := json.Unmarshal(pollRecorder.Body.Bytes(), &pollResponse); err != nil {
				t.Fatalf("decode poll response: %v", err)
			}
			if strings.Contains(pollRecorder.Body.String(), "CK=fixture") || strings.Contains(pollRecorder.Body.String(), "cookie") {
				t.Fatalf("poll response leaked credential: %s", pollRecorder.Body.String())
			}
			if pollResponse.Platform != platform || pollResponse.State != thirdparty.QRLoginStateSucceeded {
				t.Fatalf("unexpected poll response: %#v", pollResponse)
			}
			if pollResponse.Account == nil || pollResponse.Account.AccountID != "123456" || pollResponse.Account.Profile == nil || pollResponse.Account.Profile.UID != "123456" {
				t.Fatalf("unexpected poll account: %#v", pollResponse.Account)
			}
		})
	}
}

func TestThirdPartyQRCodeLoginHandlerUnknownLoginID(t *testing.T) {
	t.Parallel()

	qrLogin := &stubThirdPartyQRCodeLogin{pollErr: thirdparty.ErrQRLoginSessionNotFound}
	handler := NewThirdPartyHandlers(nil, nil, qrLogin)
	router := thirdPartyQRCodeLoginRouter(handler)
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/accounts/weibo/login/qrcode/missing", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), thirdPartyCodeInvalidRequest) {
		t.Fatalf("expected error code %q in body: %s", thirdPartyCodeInvalidRequest, recorder.Body.String())
	}
}

func TestThirdPartyQRCodeLoginHandlerDoesNotExposeRawError(t *testing.T) {
	t.Parallel()

	qrLogin := &stubThirdPartyQRCodeLogin{pollErr: errors.New("douyin qrcode poll failed: Cookie SESSDATA=secret")}
	handler := NewThirdPartyHandlers(nil, nil, qrLogin)
	router := thirdPartyQRCodeLoginRouter(handler)
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/accounts/douyin/login/qrcode/douyin_qr_fixture", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, leaked := range []string{"SESSDATA", "secret", "Cookie"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("error response leaked %q: %s", leaked, body)
		}
	}
	if !strings.Contains(body, "platform.upstream_request_failed") {
		t.Fatalf("expected upstream error code in body: %s", body)
	}
}

func TestThirdPartyQRCodeLoginHandlerCancelsSession(t *testing.T) {
	t.Parallel()

	qrLogin := &stubThirdPartyQRCodeLogin{}
	handler := NewThirdPartyHandlers(nil, nil, qrLogin)
	router := thirdPartyQRCodeLoginRouter(handler)
	request := httptest.NewRequest(http.MethodDelete, "/api/third-party/accounts/douyin/login/qrcode/douyin_qr_fixture", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 body=%s", recorder.Code, recorder.Body.String())
	}
	if len(qrLogin.cancelled) != 1 || qrLogin.cancelled[0] != "douyin:douyin_qr_fixture" {
		t.Fatalf("cancelled = %#v", qrLogin.cancelled)
	}
}

func TestThirdPartyQRCodeLoginHandlerProjectsNewStatesWithoutCredentials(t *testing.T) {
	t.Parallel()

	for _, state := range []string{thirdparty.QRLoginStateVerificationRequired, thirdparty.QRLoginStateFailed} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			qrLogin := &stubThirdPartyQRCodeLogin{pollResult: thirdparty.QRLoginPollResult{
				Platform:  thirdparty.PlatformDouyin,
				LoginID:   "douyin_qr_fixture",
				State:     state,
				ExpiresAt: time.Date(2026, 8, 21, 8, 3, 0, 0, time.UTC),
				Cookie:    "sessionid=must-not-leak;",
			}}
			handler := NewThirdPartyHandlers(nil, nil, qrLogin)
			router := thirdPartyQRCodeLoginRouter(handler)
			request := httptest.NewRequest(http.MethodGet, "/api/third-party/accounts/douyin/login/qrcode/douyin_qr_fixture", nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"state":"`+state+`"`) {
				t.Fatalf("response did not include state %q: %s", state, recorder.Body.String())
			}
			for _, leaked := range []string{"sessionid", "must-not-leak", "cookie"} {
				if strings.Contains(strings.ToLower(recorder.Body.String()), strings.ToLower(leaked)) {
					t.Fatalf("response leaked %q: %s", leaked, recorder.Body.String())
				}
			}
		})
	}
}

func TestThirdPartyQRCodeLoginHandlerMapsMissingBrowser(t *testing.T) {
	t.Parallel()

	qrLogin := &stubThirdPartyQRCodeLogin{createErr: thirdparty.ErrQRLoginBrowserUnavailable}
	handler := NewThirdPartyHandlers(nil, nil, qrLogin)
	router := thirdPartyQRCodeLoginRouter(handler)
	request := httptest.NewRequest(http.MethodPost, "/api/third-party/accounts/douyin/login/qrcode", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "platform.resource_missing") {
		t.Fatalf("response missing resource error code: %s", recorder.Body.String())
	}
}

func thirdPartyQRCodeLoginRouter(handler *ThirdPartyHandlers) chi.Router {
	router := chi.NewRouter()
	router.Post("/api/third-party/accounts/{platform}/login/qrcode", handler.HandleThirdPartyQRCodeLoginCreate())
	router.Get("/api/third-party/accounts/{platform}/login/qrcode/{login_id}", handler.HandleThirdPartyQRCodeLoginPoll())
	router.Delete("/api/third-party/accounts/{platform}/login/qrcode/{login_id}", handler.HandleThirdPartyQRCodeLoginCancel())
	return router
}
