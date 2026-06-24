package thirdpartyapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type stubThirdPartyQRCodeLogin struct {
	createResult common.CreateResult
	createErr    error
	pollResult   common.PollResult
	pollErr      error
}

func (s *stubThirdPartyQRCodeLogin) Create(context.Context, string) (common.CreateResult, error) {
	return s.createResult, s.createErr
}

func (s *stubThirdPartyQRCodeLogin) Poll(context.Context, string, string) (common.PollResult, error) {
	return s.pollResult, s.pollErr
}

func TestThirdPartyQRCodeLoginHandlersCreateAndPoll(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 6, 8, 8, 3, 0, 0, time.UTC)
	for _, platform := range []string{thirdparty.PlatformWeibo, thirdparty.PlatformDouyin, thirdparty.PlatformNeteaseMusic} {
		t.Run(platform, func(t *testing.T) {
			t.Parallel()
			qrLogin := &stubThirdPartyQRCodeLogin{
				createResult: common.CreateResult{
					Platform:  platform,
					LoginID:   platform + "_qr_fixture",
					QRCodeURL: "https://example.test/" + platform,
					ExpiresAt: expiresAt,
					State:     common.StatePendingScan,
				},
				pollResult: common.PollResult{
					Platform:  platform,
					LoginID:   platform + "_qr_fixture",
					State:     common.StateSucceeded,
					ExpiresAt: expiresAt,
					Cookie:    "CK=fixture;",
					Account: thirdparty.AccountProfile{
						UID:       "123456",
						Nickname:  "扫码账号",
						AvatarURL: "https://example.test/avatar.jpg",
					},
				},
			}
			handler := NewThirdPartyHandlers(nil, nil, qrLogin, nil, nil)
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
			if createResponse.Platform != platform || createResponse.State != common.StatePendingScan || createResponse.QRCodeURL == "" {
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
			if pollResponse.Platform != platform || pollResponse.State != common.StateSucceeded || pollResponse.Cookie == nil || *pollResponse.Cookie != "CK=fixture;" {
				t.Fatalf("unexpected poll response: %#v", pollResponse)
			}
			if pollResponse.Account == nil || pollResponse.Account.UID != "123456" {
				t.Fatalf("unexpected poll account: %#v", pollResponse.Account)
			}
		})
	}
}

func TestThirdPartyQRCodeLoginHandlerUnknownLoginID(t *testing.T) {
	t.Parallel()

	qrLogin := &stubThirdPartyQRCodeLogin{pollErr: common.ErrLoginSessionNotFound}
	handler := NewThirdPartyHandlers(nil, nil, qrLogin, nil, nil)
	router := thirdPartyQRCodeLoginRouter(handler)
	request := httptest.NewRequest(http.MethodGet, "/api/third-party/accounts/weibo/login/qrcode/missing", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), codeInvalidRequest) {
		t.Fatalf("expected error code %q in body: %s", codeInvalidRequest, recorder.Body.String())
	}
}

func thirdPartyQRCodeLoginRouter(handler *ThirdPartyHandlers) chi.Router {
	router := chi.NewRouter()
	router.Post("/api/third-party/accounts/{platform}/login/qrcode", handler.HandleThirdPartyQRCodeLoginCreate())
	router.Get("/api/third-party/accounts/{platform}/login/qrcode/{login_id}", handler.HandleThirdPartyQRCodeLoginPoll())
	return router
}
