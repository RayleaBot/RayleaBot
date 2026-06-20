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

	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type stubThirdPartyAccounts struct {
	upsertRequest thirdparty.UpsertRequest
}

func (s *stubThirdPartyAccounts) List(context.Context) ([]thirdparty.Account, error) {
	return nil, nil
}

func (s *stubThirdPartyAccounts) Upsert(_ context.Context, request thirdparty.UpsertRequest) (thirdparty.Account, error) {
	s.upsertRequest = request
	checkedAt := time.Date(2026, 6, 8, 8, 1, 1, 0, time.UTC)
	return thirdparty.Account{
		Platform:   request.Platform,
		AccountID:  request.AccountID,
		Label:      request.Label,
		Enabled:    request.Enabled,
		Configured: strings.TrimSpace(request.Cookie) != "",
		Credential: thirdparty.CredentialStatus{
			State:     thirdparty.CredentialUnknown,
			CheckedAt: &checkedAt,
		},
		UpdatedAt: time.Date(2026, 6, 8, 8, 1, 0, 0, time.UTC),
	}, nil
}

func (s *stubThirdPartyAccounts) Delete(context.Context, string, string) error {
	return nil
}

func TestThirdPartyAccountUpsertAcceptsWeiboCookie(t *testing.T) {
	t.Parallel()

	accounts := &stubThirdPartyAccounts{}
	handler := NewThirdPartyHandlers(accounts, nil, nil, nil, nil)
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
	if accounts.upsertRequest.Validate != nil {
		t.Fatal("weibo account upsert must not use Bilibili cookie validator")
	}
	var response thirdPartyAccountUpsertResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode upsert response: %v", err)
	}
	if response.Account.Platform != thirdparty.PlatformWeibo || !response.Account.Configured || response.Account.Credential.State != thirdparty.CredentialUnknown {
		t.Fatalf("unexpected upsert response: %#v", response.Account)
	}
}
