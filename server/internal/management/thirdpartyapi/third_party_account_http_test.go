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

	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

type stubThirdPartyAccounts struct {
	upsertRequest thirdparty.UpsertRequest
}

func (s *stubThirdPartyAccounts) List(context.Context) ([]thirdparty.Account, error) {
	return nil, nil
}

func (s *stubThirdPartyAccounts) Upsert(ctx context.Context, request thirdparty.UpsertRequest) (thirdparty.Account, error) {
	s.upsertRequest = request
	checkedAt := time.Date(2026, 6, 8, 8, 1, 1, 0, time.UTC)
	profile := request.Profile
	credential := thirdparty.CredentialStatus{
		State:     thirdparty.CredentialUnknown,
		CheckedAt: &checkedAt,
	}
	if request.Validate != nil && strings.TrimSpace(request.Cookie) != "" {
		checkedProfile, checkedCredential, _ := request.Validate(ctx, request.Cookie)
		profile = checkedProfile
		credential = checkedCredential
	}
	return thirdparty.Account{
		Platform:   request.Platform,
		AccountID:  request.AccountID,
		Label:      request.Label,
		Enabled:    request.Enabled,
		Configured: strings.TrimSpace(request.Cookie) != "",
		Profile:    profile,
		Credential: credential,
		UpdatedAt:  time.Date(2026, 6, 8, 8, 1, 0, 0, time.UTC),
	}, nil
}

func (s *stubThirdPartyAccounts) Delete(context.Context, string, string) error {
	return nil
}

type stubThirdPartyCredentialValidator struct {
	profile thirdparty.AccountProfile
	status  thirdparty.CredentialStatus
}

func (s stubThirdPartyCredentialValidator) CheckCookie(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	return s.profile, s.status, nil
}

func TestThirdPartyAccountUpsertAcceptsWeiboCookie(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyAccounts{}
	checkedAt := time.Date(2026, 6, 8, 8, 1, 1, 0, time.UTC)
	validator := stubThirdPartyCredentialValidator{
		profile: thirdparty.AccountProfile{
			UID:       "123456",
			Nickname:  "微博用户",
			AvatarURL: "https://weibo.com/avatar.jpg",
		},
		status: thirdparty.CredentialStatus{
			State:     thirdparty.CredentialValid,
			CheckedAt: &checkedAt,
		},
	}
	handler := NewThirdPartyHandlers(accounts, validator, nil)
	router := chi.NewRouter()
	router.Put("/api/third-party/accounts/{platform}/{account_id}", handler.HandleThirdPartyAccountUpsert())
	request := httptest.NewRequest(http.MethodPut, "/api/third-party/accounts/weibo/primary", strings.NewReader(`{"label":"微博主账号","enabled":true,"cookie":"SUB=fixture;"}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("upsert status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	if accounts.upsertRequest.Platform != thirdparty.PlatformWeibo || accounts.upsertRequest.AccountID != "primary" {
		t.Fatalf("unexpected upsert request: %#v", accounts.upsertRequest)
	}
	if accounts.upsertRequest.Validate == nil {
		t.Fatal("weibo account upsert missing platform cookie validator")
	}
	if !accounts.upsertRequest.Profile.Empty() {
		t.Fatalf("unexpected request profile: %#v", accounts.upsertRequest.Profile)
	}
	var response thirdPartyAccountUpsertResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode upsert response: %v", err)
	}
	if response.Account.Platform != thirdparty.PlatformWeibo || !response.Account.Configured || response.Account.Credential.State != thirdparty.CredentialValid {
		t.Fatalf("unexpected upsert response: %#v", response.Account)
	}
	if response.Account.Profile.UID != "123456" || response.Account.Profile.Nickname != "微博用户" || response.Account.Profile.AvatarURL == "" {
		t.Fatalf("unexpected weibo account profile: %#v", response.Account.Profile)
	}
}

func TestThirdPartyAccountUpsertRejectsDisplayOnlyFields(t *testing.T) {
	t.Parallel()

	handler := NewThirdPartyHandlers(&stubThirdPartyAccounts{}, nil, nil)
	router := chi.NewRouter()
	router.Put("/api/third-party/accounts/{platform}/{account_id}", handler.HandleThirdPartyAccountUpsert())
	request := httptest.NewRequest(http.MethodPut, "/api/third-party/accounts/weibo/primary", strings.NewReader(`{"label":"微博主账号","enabled":true,"cookie":"SUB=fixture;","profile":{"uid":"654321"}}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("upsert status = %d, want 400 body=%s", recorder.Code, recorder.Body.String())
	}
}
