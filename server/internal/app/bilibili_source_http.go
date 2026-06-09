package app

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

type bilibiliSourceHTTPHandlers struct {
	source  *source.Source
	qrLogin *source.QRLoginService
}

func newBilibiliSourceHTTPHandlers(source *source.Source, qrLogin *source.QRLoginService) *bilibiliSourceHTTPHandlers {
	return &bilibiliSourceHTTPHandlers{source: source, qrLogin: qrLogin}
}

func (h *bilibiliSourceHTTPHandlers) handleBilibiliSourceStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, bilibiliSourceStatusResponseFrom(h.source.Status(r.Context())))
	}
}

func (h *bilibiliSourceHTTPHandlers) handleBilibiliSourceRestart() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, bilibiliSourceRestartResponse{
			Accepted: true,
			Status:   bilibiliSourceStatusResponseFrom(h.source.Restart()),
		})
	}
}

func (h *bilibiliSourceHTTPHandlers) handleBilibiliQRCodeLoginCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.qrLogin == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "Bilibili 扫码登录不可用", "errors.platform.internal_error", nil)
			return
		}
		session, err := h.qrLogin.Create(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "Bilibili 扫码登录创建失败", "errors.platform.internal_error", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, bilibiliQRCodeLoginCreateResponse{
			LoginID:   session.LoginID,
			QRCodeURL: session.QRCodeURL,
			ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
			State:     session.State,
		})
	}
}

func (h *bilibiliSourceHTTPHandlers) handleBilibiliQRCodeLoginPoll() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.qrLogin == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "Bilibili 扫码登录不可用", "errors.platform.internal_error", nil)
			return
		}
		session, err := h.qrLogin.Poll(r.Context(), chi.URLParam(r, "login_id"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "Bilibili 扫码登录状态不可用", "errors.platform.invalid_request", nil)
			return
		}
		var cookie *string
		if session.Cookie != "" {
			cookie = &session.Cookie
		}
		var account *thirdPartyAccountProfile
		if session.Account.UID != "" || session.Account.Nickname != "" || session.Account.AvatarURL != "" {
			account = &thirdPartyAccountProfile{
				UID:       session.Account.UID,
				Nickname:  session.Account.Nickname,
				AvatarURL: session.Account.AvatarURL,
			}
		}
		httpapi.WriteJSON(w, http.StatusOK, bilibiliQRCodeLoginPollResponse{
			LoginID:   session.LoginID,
			State:     session.State,
			ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
			Cookie:    cookie,
			Account:   account,
		})
	}
}

type bilibiliSourceStatusResponse struct {
	Status    string                      `json:"status"`
	Summary   string                      `json:"summary"`
	Live      bilibiliSourceLiveStatus    `json:"live"`
	Dynamic   bilibiliSourceDynamicStatus `json:"dynamic"`
	Diagnosis bilibiliSourceDiagnosis     `json:"diagnosis"`
	Accounts  []thirdPartyAccountSummary  `json:"accounts"`
}

type bilibiliSourceLiveStatus struct {
	WatchedRooms    int     `json:"watched_rooms"`
	ConnectedRooms  int     `json:"connected_rooms"`
	FailedRooms     int     `json:"failed_rooms"`
	FallbackPolling bool    `json:"fallback_polling"`
	LastEventAt     *string `json:"last_event_at"`
	LastError       string  `json:"last_error"`
}

type bilibiliSourceDynamicStatus struct {
	Enabled         bool    `json:"enabled"`
	IntervalSeconds int     `json:"interval_seconds"`
	WatchedUIDs     int     `json:"watched_uids"`
	AutoFollow      bool    `json:"auto_follow"`
	LastPollAt      *string `json:"last_poll_at"`
	LastEventAt     *string `json:"last_event_at"`
	LastError       string  `json:"last_error"`
}

type bilibiliSourceRestartResponse struct {
	Accepted bool                         `json:"accepted"`
	Status   bilibiliSourceStatusResponse `json:"status"`
}

type bilibiliSourceDiagnosis struct {
	Level       string                          `json:"level"`
	Headline    string                          `json:"headline"`
	Description string                          `json:"description"`
	Causes      []bilibiliSourceDiagnosisCause  `json:"causes"`
	Impacts     []string                        `json:"impacts"`
	Actions     []bilibiliSourceDiagnosisAction `json:"actions"`
	UpdatedAt   string                          `json:"updated_at"`
}

type bilibiliSourceDiagnosisCause struct {
	Scope     string  `json:"scope"`
	Code      string  `json:"code"`
	Title     string  `json:"title"`
	Detail    string  `json:"detail"`
	LastError string  `json:"last_error"`
	RetryAt   *string `json:"retry_at"`
}

type bilibiliSourceDiagnosisAction struct {
	Kind    string  `json:"kind"`
	Label   string  `json:"label"`
	Target  *string `json:"target"`
	Primary bool    `json:"primary"`
}

type bilibiliQRCodeLoginCreateResponse struct {
	LoginID   string `json:"login_id"`
	QRCodeURL string `json:"qrcode_url"`
	ExpiresAt string `json:"expires_at"`
	State     string `json:"state"`
}

type bilibiliQRCodeLoginPollResponse struct {
	LoginID   string                    `json:"login_id"`
	State     string                    `json:"state"`
	ExpiresAt string                    `json:"expires_at"`
	Cookie    *string                   `json:"cookie"`
	Account   *thirdPartyAccountProfile `json:"account"`
}

func bilibiliSourceStatusResponseFrom(status source.Status) bilibiliSourceStatusResponse {
	return bilibiliSourceStatusResponse{
		Status:  status.Status,
		Summary: status.Summary,
		Live: bilibiliSourceLiveStatus{
			WatchedRooms:    status.Live.WatchedRooms,
			ConnectedRooms:  status.Live.ConnectedRooms,
			FailedRooms:     status.Live.FailedRooms,
			FallbackPolling: status.Live.FallbackPolling,
			LastEventAt:     timeStringPtr(status.Live.LastEventAt),
			LastError:       status.Live.LastError,
		},
		Dynamic: bilibiliSourceDynamicStatus{
			Enabled:         status.Dynamic.Enabled,
			IntervalSeconds: status.Dynamic.IntervalSeconds,
			WatchedUIDs:     status.Dynamic.WatchedUIDs,
			AutoFollow:      status.Dynamic.AutoFollow,
			LastPollAt:      timeStringPtr(status.Dynamic.LastPollAt),
			LastEventAt:     timeStringPtr(status.Dynamic.LastEventAt),
			LastError:       status.Dynamic.LastError,
		},
		Diagnosis: bilibiliSourceDiagnosisFrom(status),
		Accounts:  accountSummaries(status.Accounts),
	}
}

func bilibiliSourceDiagnosisFrom(status source.Status) bilibiliSourceDiagnosis {
	diagnosis := status.Diagnosis
	if diagnosis.Level == "" || diagnosis.Headline == "" || diagnosis.UpdatedAt.IsZero() {
		diagnosis = source.DiagnosisForStatus(status, time.Now().UTC())
	}
	causes := make([]bilibiliSourceDiagnosisCause, 0, len(diagnosis.Causes))
	for _, cause := range diagnosis.Causes {
		causes = append(causes, bilibiliSourceDiagnosisCause{
			Scope:     cause.Scope,
			Code:      cause.Code,
			Title:     cause.Title,
			Detail:    cause.Detail,
			LastError: cause.LastError,
			RetryAt:   timeStringPtr(cause.RetryAt),
		})
	}
	actions := make([]bilibiliSourceDiagnosisAction, 0, len(diagnosis.Actions))
	for _, action := range diagnosis.Actions {
		actions = append(actions, bilibiliSourceDiagnosisAction{
			Kind:    action.Kind,
			Label:   action.Label,
			Target:  action.Target,
			Primary: action.Primary,
		})
	}
	return bilibiliSourceDiagnosis{
		Level:       diagnosis.Level,
		Headline:    diagnosis.Headline,
		Description: diagnosis.Description,
		Causes:      causes,
		Impacts:     append([]string(nil), diagnosis.Impacts...),
		Actions:     actions,
		UpdatedAt:   diagnosis.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func timeStringPtr(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	text := value.UTC().Format(time.RFC3339)
	return &text
}
