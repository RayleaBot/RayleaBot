package managementhttp

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/thirdparty"
)

func (h *ThirdPartyHandlers) HandleThirdPartyAccountList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		accounts, err := h.accounts.List(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "三方账号读取失败", "errors.platform.internal_error", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, thirdPartyAccountsResponse{Items: accountSummaries(accounts)})
	}
}

func (h *ThirdPartyHandlers) HandleThirdPartyAccountUpsert() http.HandlerFunc {
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
				if h.accountValidator == nil {
					return thirdparty.AccountProfile{}, thirdparty.CredentialStatus{}, nil
				}
				return h.accountValidator.CheckCookie(ctx, cookie)
			},
		})
		if err != nil {
			writeThirdPartyAccountError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, thirdPartyAccountUpsertResponse{Account: accountSummary(account)})
	}
}

func (h *ThirdPartyHandlers) HandleThirdPartyAccountDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h.accounts.Delete(r.Context(), chi.URLParam(r, "platform"), chi.URLParam(r, "account_id")); err != nil {
			writeThirdPartyAccountError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
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
