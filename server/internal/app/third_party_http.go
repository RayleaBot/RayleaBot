package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

type thirdPartyHTTPHandlers struct {
	accounts      *thirdparty.Service
	accountClient *source.AccountClient
}

func newThirdPartyHTTPHandlers(accounts *thirdparty.Service, accountClient *source.AccountClient) *thirdPartyHTTPHandlers {
	return &thirdPartyHTTPHandlers{accounts: accounts, accountClient: accountClient}
}

func (h *thirdPartyHTTPHandlers) handleThirdPartyAccountList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accounts, err := h.accounts.List(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "三方账号读取失败", "errors.platform.internal_error", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, thirdPartyAccountsResponse{Items: accountSummaries(accounts)})
	}
}

func (h *thirdPartyHTTPHandlers) handleThirdPartyAccountUpsert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body thirdPartyAccountUpsertRequest
		if err := httpapi.DecodeStrictJSON(w, r, &body, httpapi.MaxManagementJSONBodyBytes); err != nil || body.Label == nil || body.Enabled == nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求格式不正确", "errors.platform.invalid_request", nil)
			return
		}
		account, err := h.accounts.Upsert(r.Context(), thirdparty.UpsertRequest{
			Platform:  chi.URLParam(r, "platform"),
			AccountID: chi.URLParam(r, "account_id"),
			Label:     *body.Label,
			Enabled:   *body.Enabled,
			Cookie:    body.Cookie,
			Validate: func(ctx context.Context, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
				if h.accountClient == nil {
					return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, nil
				}
				return h.accountClient.CheckCookie(ctx, cookie)
			},
		})
		if err != nil {
			writeThirdPartyAccountError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, thirdPartyAccountUpsertResponse{Account: accountSummary(account)})
	}
}

func (h *thirdPartyHTTPHandlers) handleThirdPartyAccountDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h.accounts.Delete(r.Context(), chi.URLParam(r, "platform"), chi.URLParam(r, "account_id")); err != nil {
			writeThirdPartyAccountError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type thirdPartyAccountsResponse struct {
	Items []thirdPartyAccountSummary `json:"items"`
}

type thirdPartyAccountUpsertRequest struct {
	Label   *string `json:"label"`
	Enabled *bool   `json:"enabled"`
	Cookie  string  `json:"cookie,omitempty"`
}

type thirdPartyAccountUpsertResponse struct {
	Account thirdPartyAccountSummary `json:"account"`
}

type thirdPartyAccountSummary struct {
	Platform   string                         `json:"platform"`
	AccountID  string                         `json:"account_id"`
	Label      string                         `json:"label"`
	Enabled    bool                           `json:"enabled"`
	Configured bool                           `json:"configured"`
	Profile    *thirdPartyAccountProfile      `json:"profile"`
	Credential thirdPartyCredentialStatus     `json:"credential"`
	Polling    thirdPartyAccountPollingStatus `json:"polling"`
	UpdatedAt  string                         `json:"updated_at"`
}

type thirdPartyAccountProfile struct {
	UID       string `json:"uid"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type thirdPartyCredentialStatus struct {
	State     string  `json:"state"`
	CheckedAt *string `json:"checked_at"`
	LastError string  `json:"last_error"`
}

type thirdPartyAccountPollingStatus struct {
	Enabled    bool    `json:"enabled"`
	LastUsedAt *string `json:"last_used_at"`
}

func accountSummaries(accounts []thirdparty.Account) []thirdPartyAccountSummary {
	items := make([]thirdPartyAccountSummary, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, accountSummary(account))
	}
	return items
}

func accountSummary(account thirdparty.Account) thirdPartyAccountSummary {
	updatedAt := ""
	if !account.UpdatedAt.IsZero() {
		updatedAt = account.UpdatedAt.UTC().Format(time.RFC3339)
	}
	var profile *thirdPartyAccountProfile
	if strings.TrimSpace(account.Profile.UID) != "" || strings.TrimSpace(account.Profile.Nickname) != "" || strings.TrimSpace(account.Profile.AvatarURL) != "" {
		profile = &thirdPartyAccountProfile{
			UID:       account.Profile.UID,
			Nickname:  account.Profile.Nickname,
			AvatarURL: account.Profile.AvatarURL,
		}
	}
	return thirdPartyAccountSummary{
		Platform:   account.Platform,
		AccountID:  account.AccountID,
		Label:      account.Label,
		Enabled:    account.Enabled,
		Configured: account.Configured,
		Profile:    profile,
		Credential: thirdPartyCredentialStatus{
			State:     account.Credential.State,
			CheckedAt: timeStringPtr(account.Credential.CheckedAt),
			LastError: account.Credential.LastError,
		},
		Polling: thirdPartyAccountPollingStatus{
			Enabled:    account.Enabled && account.Configured,
			LastUsedAt: timeStringPtr(account.LastUsedAt),
		},
		UpdatedAt: updatedAt,
	}
}

func writeThirdPartyAccountError(w http.ResponseWriter, r *http.Request, err error) {
	message := strings.TrimSpace(err.Error())
	if errors.Is(err, thirdparty.ErrInvalidAccount) || strings.Contains(message, "unsupported third-party platform") || strings.Contains(message, "invalid third-party account id") {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "三方账号参数不正确", "errors.platform.invalid_request", nil)
		return
	}
	httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "三方账号保存失败", "errors.platform.internal_error", nil)
}
