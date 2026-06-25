package bilibiliapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/qrcode"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/go-chi/chi/v5"
)

type stubBilibiliQRCodeLogin struct {
	createResult qrcode.CreateResult
	createErr    error
	pollResult   qrcode.PollResult
	pollErr      error
}

func (s *stubBilibiliQRCodeLogin) Create(context.Context, string) (qrcode.CreateResult, error) {
	return s.createResult, s.createErr
}

func (s *stubBilibiliQRCodeLogin) Poll(context.Context, string, string) (qrcode.PollResult, error) {
	return s.pollResult, s.pollErr
}

func TestBilibiliQRCodeLoginHandlerDoesNotReturnCookie(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 6, 8, 8, 3, 0, 0, time.UTC)
	qrLogin := &stubBilibiliQRCodeLogin{
		createResult: qrcode.CreateResult{
			LoginID:   "qr_fixture",
			QRCodeURL: "https://passport.bilibili.com/scan?qrcode_key=fixture",
			ExpiresAt: expiresAt,
			State:     qrcode.StatePendingScan,
		},
		pollResult: qrcode.PollResult{
			LoginID:   "qr_fixture",
			State:     qrcode.StateSucceeded,
			ExpiresAt: expiresAt,
			Cookie:    "SESSDATA=fixture; bili_jct=fixture;",
			Account: thirdparty.AccountProfile{
				UID:       "123456",
				Nickname:  "扫码账号",
				AvatarURL: "https://example.test/avatar.jpg",
			},
			SavedAccount: &thirdparty.Account{
				Platform:   thirdparty.PlatformBilibili,
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
	handler := NewBilibiliHandlers(nil, qrLogin, nil)
	router := bilibiliQRCodeLoginRouter(handler)

	pollRecorder := httptest.NewRecorder()
	pollRequest := httptest.NewRequest(http.MethodGet, "/api/bilibili/login/qrcode/qr_fixture", nil)
	router.ServeHTTP(pollRecorder, pollRequest)

	if pollRecorder.Code != http.StatusOK {
		t.Fatalf("poll status = %d, want 200 body=%s", pollRecorder.Code, pollRecorder.Body.String())
	}
	body := pollRecorder.Body.String()
	for _, leaked := range []string{"SESSDATA", "bili_jct", "cookie"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("poll response leaked %q: %s", leaked, body)
		}
	}
	var response bilibiliQRCodeLoginPollResponse
	if err := json.Unmarshal(pollRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode poll response: %v", err)
	}
	if response.Account == nil || response.Account.AccountID != "123456" || response.Account.Profile == nil || response.Account.Profile.UID != "123456" {
		t.Fatalf("unexpected account response: %#v", response.Account)
	}
}

func TestBilibiliQRCodeLoginHandlerDoesNotExposeRawError(t *testing.T) {
	t.Parallel()

	qrLogin := &stubBilibiliQRCodeLogin{pollErr: errors.New("bilibili qr poll failed: SESSDATA=secret")}
	handler := NewBilibiliHandlers(nil, qrLogin, nil)
	router := bilibiliQRCodeLoginRouter(handler)
	request := httptest.NewRequest(http.MethodGet, "/api/bilibili/login/qrcode/qr_fixture", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 body=%s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, leaked := range []string{"SESSDATA", "secret"} {
		if strings.Contains(body, leaked) {
			t.Fatalf("error response leaked %q: %s", leaked, body)
		}
	}
	if !strings.Contains(body, "platform.upstream_request_failed") {
		t.Fatalf("expected upstream error code in body: %s", body)
	}
}

func bilibiliQRCodeLoginRouter(handler *BilibiliHandlers) chi.Router {
	router := chi.NewRouter()
	router.Get("/api/bilibili/login/qrcode/{login_id}", handler.HandleBilibiliQRCodeLoginPoll())
	return router
}
