package management

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
	"github.com/go-chi/chi/v5"
)

const (
	thirdPartyCodeInvalidRequest = "platform.invalid_request"
	thirdPartyCodeInternalError  = "platform.internal_error"
)

type ThirdPartyHandlers struct {
	accounts         thirdPartyAccountService
	accountValidator thirdPartyCredentialValidator
	qrLogin          thirdPartyQRCodeLoginService
}

type thirdPartyAccountService interface {
	List(context.Context) ([]thirdparty.Account, error)
	Upsert(context.Context, thirdparty.UpsertRequest) (thirdparty.Account, error)
	Delete(context.Context, string, string) error
}

type thirdPartyCredentialValidator interface {
	CheckCookie(context.Context, string, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error)
}

type thirdPartyQRCodeLoginService interface {
	Create(context.Context, string) (thirdparty.QRLoginCreateResult, error)
	Poll(context.Context, string, string) (thirdparty.QRLoginPollResult, error)
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
	Platform   string                     `json:"platform"`
	AccountID  string                     `json:"account_id"`
	Label      string                     `json:"label"`
	Enabled    bool                       `json:"enabled"`
	Configured bool                       `json:"configured"`
	Profile    *thirdPartyAccountProfile  `json:"profile"`
	Credential thirdPartyCredentialStatus `json:"credential"`
	UpdatedAt  string                     `json:"updated_at"`
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

type thirdPartyQRCodeLoginCreateResponse struct {
	Platform  string `json:"platform"`
	LoginID   string `json:"login_id"`
	QRCodeURL string `json:"qrcode_url"`
	ExpiresAt string `json:"expires_at"`
	State     string `json:"state"`
}

type thirdPartyQRCodeLoginPollResponse struct {
	Platform  string                    `json:"platform"`
	LoginID   string                    `json:"login_id"`
	State     string                    `json:"state"`
	ExpiresAt string                    `json:"expires_at"`
	Account   *thirdPartyAccountSummary `json:"account"`
}

func NewThirdPartyHandlers(accounts thirdPartyAccountService, accountValidator thirdPartyCredentialValidator, qrLogin thirdPartyQRCodeLoginService) *ThirdPartyHandlers {
	return &ThirdPartyHandlers{
		accounts:         accounts,
		accountValidator: accountValidator,
		qrLogin:          qrLogin,
	}
}

func (h *ThirdPartyHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/third-party/accounts", h.HandleThirdPartyAccountList())
	router.Post("/api/third-party/accounts/{platform}/login/qrcode", h.HandleThirdPartyQRCodeLoginCreate())
	router.Get("/api/third-party/accounts/{platform}/login/qrcode/{login_id}", h.HandleThirdPartyQRCodeLoginPoll())
	router.Put("/api/third-party/accounts/{platform}/{account_id}", h.HandleThirdPartyAccountUpsert())
	router.Delete("/api/third-party/accounts/{platform}/{account_id}", h.HandleThirdPartyAccountDelete())
}

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
			Validate:  h.credentialValidator(chi.URLParam(r, "platform")),
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

func (h *ThirdPartyHandlers) credentialValidator(platform string) func(context.Context, string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
	normalized, err := thirdparty.NormalizePlatform(platform)
	if err != nil {
		return nil
	}
	if h.accountValidator == nil {
		return nil
	}
	return func(ctx context.Context, cookie string) (thirdparty.AccountProfile, thirdparty.CredentialStatus, error) {
		return h.accountValidator.CheckCookie(ctx, normalized, cookie)
	}
}

func writeThirdPartyAccountError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, thirdparty.ErrInvalidAccount) {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "三方账号参数不正确", "errors.platform.invalid_request", nil)
		return
	}
	httpapi.WriteDomainError(w, r, &httpapi.DomainError{
		Code:        "platform.upstream_request_failed",
		HTTPStatus:  http.StatusBadGateway,
		SafeMessage: "三方账号保存失败",
		MessageKey:  "errors.platform.upstream_request_failed",
		Details:     map[string]any{"reason": "account_save_failed"},
		Cause:       err,
	})
}

func (h *ThirdPartyHandlers) HandleThirdPartyQRCodeLoginCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.qrLogin == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, thirdPartyCodeInternalError, "三方扫码登录不可用", "errors.platform.internal_error", nil)
			return
		}
		result, err := h.qrLogin.Create(r.Context(), chi.URLParam(r, "platform"))
		if err != nil {
			writeThirdPartyQRCodeLoginError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, thirdPartyQRCodeLoginCreateResponseFrom(result))
	}
}

func (h *ThirdPartyHandlers) HandleThirdPartyQRCodeLoginPoll() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.qrLogin == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, thirdPartyCodeInternalError, "三方扫码登录不可用", "errors.platform.internal_error", nil)
			return
		}
		result, err := h.qrLogin.Poll(r.Context(), chi.URLParam(r, "platform"), chi.URLParam(r, "login_id"))
		if err != nil {
			writeThirdPartyQRCodeLoginError(w, r, err)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, thirdPartyQRCodeLoginPollResponseFrom(result))
	}
}

func thirdPartyQRCodeLoginCreateResponseFrom(result thirdparty.QRLoginCreateResult) thirdPartyQRCodeLoginCreateResponse {
	return thirdPartyQRCodeLoginCreateResponse{
		Platform:  result.Platform,
		LoginID:   result.LoginID,
		QRCodeURL: result.QRCodeURL,
		ExpiresAt: thirdPartyTimeString(result.ExpiresAt),
		State:     result.State,
	}
}

func thirdPartyQRCodeLoginPollResponseFrom(result thirdparty.QRLoginPollResult) thirdPartyQRCodeLoginPollResponse {
	var account *thirdPartyAccountSummary
	if result.SavedAccount != nil {
		summary := accountSummary(*result.SavedAccount)
		account = &summary
	}
	return thirdPartyQRCodeLoginPollResponse{
		Platform:  result.Platform,
		LoginID:   result.LoginID,
		State:     result.State,
		ExpiresAt: thirdPartyTimeString(result.ExpiresAt),
		Account:   account,
	}
}

func writeThirdPartyQRCodeLoginError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, thirdparty.ErrInvalidAccount) || errors.Is(err, thirdparty.ErrQRLoginUnsupportedPlatform) || errors.Is(err, thirdparty.ErrQRLoginSessionNotFound) {
		httpapi.WriteError(w, r, http.StatusBadRequest, thirdPartyCodeInvalidRequest, "三方扫码登录参数不正确", "errors.platform.invalid_request", nil)
		return
	}
	httpapi.WriteDomainError(w, r, &httpapi.DomainError{
		Code:        "platform.upstream_request_failed",
		HTTPStatus:  http.StatusBadGateway,
		SafeMessage: "三方扫码登录暂时不可用",
		MessageKey:  "errors.platform.upstream_request_failed",
		Details:     map[string]any{"reason": thirdPartyQRCodeLoginErrorReason(err)},
		Cause:       err,
	})
}

func thirdPartyQRCodeLoginErrorReason(err error) string {
	if errors.Is(err, thirdparty.ErrQRLoginCredentialMissing) {
		return "credential_missing"
	}
	return "upstream_failed"
}

func accountSummaries(accounts []thirdparty.Account) []thirdPartyAccountSummary {
	items := make([]thirdPartyAccountSummary, 0, len(accounts))
	for _, account := range accounts {
		items = append(items, accountSummary(account))
	}
	return items
}

func accountSummary(account thirdparty.Account) thirdPartyAccountSummary {
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
			CheckedAt: thirdPartyTimeStringPtr(account.Credential.CheckedAt),
			LastError: account.Credential.LastError,
		},
		UpdatedAt: thirdPartyTimeString(account.UpdatedAt),
	}
}

func thirdPartyTimeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func thirdPartyTimeStringPtr(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	text := value.UTC().Format(time.RFC3339)
	return &text
}
