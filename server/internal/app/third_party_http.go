package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
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
	mediaClient   *http.Client
}

func newThirdPartyHTTPHandlers(accounts *thirdparty.Service, accountClient *source.AccountClient, transport http.RoundTripper) *thirdPartyHTTPHandlers {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &thirdPartyHTTPHandlers{
		accounts:      accounts,
		accountClient: accountClient,
		mediaClient:   &http.Client{Transport: transport, Timeout: 20 * time.Second},
	}
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

func (h *thirdPartyHTTPHandlers) handleThirdPartyMonitorList(source *source.Source) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		platform := strings.TrimSpace(r.URL.Query().Get("platform"))
		if platform == "" {
			platform = thirdparty.PlatformBilibili
		}
		if platform != thirdparty.PlatformBilibili {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "三方监控平台不正确", "errors.platform.invalid_request", nil)
			return
		}
		snapshot, err := source.MonitorSnapshot(r.Context())
		if err != nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "三方监控读取失败", "errors.platform.internal_error", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, thirdPartyMonitorsResponseFrom(snapshot))
	}
}

func (h *thirdPartyHTTPHandlers) handleThirdPartyMedia() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const maxMediaBytes = 8 << 20
		mediaURL, err := parseThirdPartyMediaURL(r.URL.Query().Get("url"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "三方媒体地址不受支持", "errors.platform.invalid_request", nil)
			return
		}
		request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, mediaURL, nil)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "三方媒体地址不受支持", "errors.platform.invalid_request", nil)
			return
		}
		request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		request.Header.Set("Referer", "https://www.bilibili.com/")
		request.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")

		client := h.mediaClient
		if client == nil {
			client = http.DefaultClient
		}
		response, err := client.Do(request)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadGateway, codeInternalError, "三方媒体读取失败", "errors.platform.internal_error", nil)
			return
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			httpapi.WriteError(w, r, http.StatusBadGateway, codeInternalError, "三方媒体读取失败", "errors.platform.internal_error", nil)
			return
		}
		contentType := normalizeThirdPartyMediaContentType(response.Header.Get("Content-Type"))
		if contentType == "" {
			httpapi.WriteError(w, r, http.StatusBadGateway, codeInternalError, "三方媒体响应格式不正确", "errors.platform.internal_error", nil)
			return
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, maxMediaBytes+1))
		if err != nil || len(body) > maxMediaBytes {
			httpapi.WriteError(w, r, http.StatusBadGateway, codeInternalError, "三方媒体读取失败", "errors.platform.internal_error", nil)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "private, max-age=3600")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
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

func parseThirdPartyMediaURL(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "https" {
		return "", errors.New("unsupported scheme")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "hdslb.com" && !strings.HasSuffix(host, ".hdslb.com") {
		return "", errors.New("unsupported host")
	}
	if parsed.User != nil || parsed.RawQuery != "" {
		return "", errors.New("unsupported media url")
	}
	path := strings.ToLower(parsed.EscapedPath())
	if path == "" || !(strings.HasPrefix(path, "/bfs/") || strings.HasPrefix(path, "/fs/")) {
		return "", errors.New("unsupported path")
	}
	return parsed.String(), nil
}

func normalizeThirdPartyMediaContentType(value string) string {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	switch contentType {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "image/avif":
		return contentType
	default:
		return ""
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

type thirdPartyMonitorsResponse struct {
	Platform  string                  `json:"platform"`
	Items     []thirdPartyMonitorItem `json:"items"`
	UpdatedAt string                  `json:"updated_at"`
}

type thirdPartyMonitorItem struct {
	UID       string                    `json:"uid"`
	Username  string                    `json:"username"`
	AvatarURL string                    `json:"avatar_url"`
	Services  []string                  `json:"services"`
	Dynamic   *thirdPartyMonitorDynamic `json:"dynamic"`
	Live      thirdPartyMonitorLive     `json:"live"`
	UpdatedAt string                    `json:"updated_at"`
}

type thirdPartyMonitorDynamic struct {
	LastID      string                   `json:"last_id"`
	Service     string                   `json:"service"`
	Title       string                   `json:"title"`
	Summary     string                   `json:"summary"`
	URL         string                   `json:"url"`
	Images      []thirdPartyMonitorImage `json:"images"`
	PublishedAt *string                  `json:"published_at"`
	ObservedAt  string                   `json:"observed_at"`
}

type thirdPartyMonitorLive struct {
	RoomID          string  `json:"room_id"`
	RoomName        string  `json:"room_name"`
	RoomURL         string  `json:"room_url"`
	CoverURL        string  `json:"cover_url"`
	IsLive          bool    `json:"is_live"`
	LiveStartedAt   *string `json:"live_started_at"`
	LiveEndedAt     *string `json:"live_ended_at"`
	ConnectionState string  `json:"connection_state"`
	LastError       string  `json:"last_error"`
	UpdatedAt       *string `json:"updated_at"`
}

type thirdPartyMonitorImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
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

func thirdPartyMonitorsResponseFrom(snapshot source.MonitorSnapshot) thirdPartyMonitorsResponse {
	items := make([]thirdPartyMonitorItem, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		items = append(items, thirdPartyMonitorItemFrom(item))
	}
	return thirdPartyMonitorsResponse{
		Platform:  snapshot.Platform,
		Items:     items,
		UpdatedAt: timeString(snapshot.UpdatedAt),
	}
}

func thirdPartyMonitorItemFrom(item source.MonitorItem) thirdPartyMonitorItem {
	services := append([]string(nil), item.Services...)
	sort.Strings(services)
	return thirdPartyMonitorItem{
		UID:       item.UID,
		Username:  item.Username,
		AvatarURL: item.AvatarURL,
		Services:  services,
		Dynamic:   thirdPartyMonitorDynamicFrom(item.Dynamic),
		Live: thirdPartyMonitorLive{
			RoomID:          item.Live.RoomID,
			RoomName:        item.Live.RoomName,
			RoomURL:         item.Live.RoomURL,
			CoverURL:        item.Live.CoverURL,
			IsLive:          item.Live.IsLive,
			LiveStartedAt:   timeStringPtr(item.Live.LiveStartedAt),
			LiveEndedAt:     timeStringPtr(item.Live.LiveEndedAt),
			ConnectionState: item.Live.ConnectionState,
			LastError:       item.Live.LastError,
			UpdatedAt:       timeStringPtr(item.Live.UpdatedAt),
		},
		UpdatedAt: timeString(item.UpdatedAt),
	}
}

func thirdPartyMonitorDynamicFrom(dynamic *source.MonitorDynamic) *thirdPartyMonitorDynamic {
	if dynamic == nil {
		return nil
	}
	images := make([]thirdPartyMonitorImage, 0, len(dynamic.Images))
	for _, image := range dynamic.Images {
		if strings.TrimSpace(image.URL) == "" {
			continue
		}
		images = append(images, thirdPartyMonitorImage{URL: image.URL, Width: image.Width, Height: image.Height})
	}
	return &thirdPartyMonitorDynamic{
		LastID:      dynamic.LastID,
		Service:     dynamic.Service,
		Title:       dynamic.Title,
		Summary:     dynamic.Summary,
		URL:         dynamic.URL,
		Images:      images,
		PublishedAt: timeStringPtr(dynamic.PublishedAt),
		ObservedAt:  timeString(dynamic.ObservedAt),
	}
}

func timeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func writeThirdPartyAccountError(w http.ResponseWriter, r *http.Request, err error) {
	message := strings.TrimSpace(err.Error())
	if errors.Is(err, thirdparty.ErrInvalidAccount) || strings.Contains(message, "unsupported third-party platform") || strings.Contains(message, "invalid third-party account id") {
		httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "三方账号参数不正确", "errors.platform.invalid_request", nil)
		return
	}
	httpapi.WriteError(w, r, http.StatusInternalServerError, "platform.internal_error", "三方账号保存失败", "errors.platform.internal_error", nil)
}
