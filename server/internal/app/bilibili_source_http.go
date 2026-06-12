package app

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	source "github.com/RayleaBot/RayleaBot/server/internal/bilibili"
	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

type bilibiliSourceHTTPHandlers struct {
	source     *source.Source
	qrLogin    *source.QRLoginService
	userClient *http.Client
}

func newBilibiliSourceHTTPHandlers(source *source.Source, qrLogin *source.QRLoginService, transport http.RoundTripper) *bilibiliSourceHTTPHandlers {
	return &bilibiliSourceHTTPHandlers{
		source:  source,
		qrLogin: qrLogin,
		userClient: &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
		},
	}
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

func (h *bilibiliSourceHTTPHandlers) handleBilibiliUserResolve() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("query"))
		if query == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		response, err := h.resolveBilibiliUser(r.Context(), query)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadGateway, "platform.upstream_request_failed", "Bilibili 用户信息读取失败", "errors.platform.upstream_request_failed", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
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

type bilibiliResolvedUser struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Fans      int    `json:"fans,omitempty"`
}

type bilibiliUserResolveResponse struct {
	Query      string                 `json:"query"`
	Exact      bool                   `json:"exact"`
	User       *bilibiliResolvedUser  `json:"user,omitempty"`
	Candidates []bilibiliResolvedUser `json:"candidates"`
	Message    string                 `json:"message,omitempty"`
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

const (
	bilibiliUserInfoURL   = "https://api.bilibili.com/x/space/acc/info?mid=%s&jsonp=jsonp"
	bilibiliUserSearchURL = "https://api.bilibili.com/x/web-interface/search/type"
)

var bilibiliHTMLTagPattern = regexp.MustCompile(`<[^>]+>`)

func (h *bilibiliSourceHTTPHandlers) resolveBilibiliUser(ctx context.Context, query string) (bilibiliUserResolveResponse, error) {
	response := bilibiliUserResolveResponse{
		Query:      query,
		Candidates: []bilibiliResolvedUser{},
	}
	if isDigits(query) {
		document, err := h.getBilibiliJSON(ctx, fmt.Sprintf(bilibiliUserInfoURL, url.QueryEscape(query)), query)
		if err != nil {
			return response, err
		}
		if message := bilibiliUserDocumentMessage(document, "没有找到这个 Bilibili 用户。"); message != "" {
			response.Message = message
			return response, nil
		}
		user, ok := bilibiliUserFromInfoDocument(document)
		if !ok {
			response.Message = "没有找到这个 Bilibili 用户。"
			return response, nil
		}
		response.Exact = true
		response.User = &user
		response.Candidates = []bilibiliResolvedUser{user}
		return response, nil
	}

	searchURL, err := bilibiliUserSearchURLFor(query)
	if err != nil {
		return response, err
	}
	document, err := h.getBilibiliJSON(ctx, searchURL, "")
	if err != nil {
		return response, err
	}
	if message := bilibiliUserDocumentMessage(document, "没有搜索到 Bilibili 用户。"); message != "" {
		response.Message = message
		return response, nil
	}
	candidates := bilibiliUsersFromSearchDocument(document)
	response.Candidates = candidates
	if len(candidates) == 0 {
		response.Message = "没有搜索到 Bilibili 用户。"
		return response, nil
	}
	for i := range candidates {
		if candidates[i].Name == query {
			response.Exact = true
			response.User = &candidates[i]
			break
		}
	}
	return response, nil
}

func (h *bilibiliSourceHTTPHandlers) getBilibiliJSON(ctx context.Context, requestURL, refererUID string) (map[string]any, error) {
	client := h.userClient
	if client == nil {
		client = http.DefaultClient
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	applyBilibiliUserResolveHeaders(request, refererUID)
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("bilibili user request failed: http %d", resp.StatusCode)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		return nil, err
	}
	return document, nil
}

func applyBilibiliUserResolveHeaders(request *http.Request, uid string) {
	request.Header.Set("Accept", "application/json, text/plain, */*")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	if uid != "" {
		request.Header.Set("Referer", "https://space.bilibili.com/"+uid+"/dynamic")
	} else {
		request.Header.Set("Referer", "https://www.bilibili.com/")
	}
}

func bilibiliUserSearchURLFor(keyword string) (string, error) {
	parsed, err := url.Parse(bilibiliUserSearchURL)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("keyword", keyword)
	query.Set("page", "1")
	query.Set("search_type", "bili_user")
	query.Set("order", "totalrank")
	query.Set("pagesize", "5")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func bilibiliUserDocumentMessage(document map[string]any, notFoundMessage string) string {
	if document == nil {
		return "Bilibili 响应格式不正确。"
	}
	code := intFromAny(document["code"])
	if code == 0 {
		return ""
	}
	switch code {
	case -404:
		return notFoundMessage
	case -412, -352:
		return "Bilibili 暂时限制了本次查询，请稍后再试。"
	default:
		message := cleanBilibiliUserText(document["message"])
		if message == "" {
			message = cleanBilibiliUserText(document["msg"])
		}
		if message == "" {
			return "Bilibili 用户信息读取失败。"
		}
		return message
	}
}

func bilibiliUserFromInfoDocument(document map[string]any) (bilibiliResolvedUser, bool) {
	data, _ := document["data"].(map[string]any)
	uid := bilibiliIDText(data["mid"])
	name := cleanBilibiliUserText(firstNonEmpty(data["name"], data["uname"]))
	if !isDigits(uid) || name == "" {
		return bilibiliResolvedUser{}, false
	}
	return bilibiliResolvedUser{
		UID:       uid,
		Name:      name,
		AvatarURL: cleanBilibiliUserURL(firstNonEmpty(data["face"], data["avatar"], data["upic"])),
		Fans:      intFromAny(data["fans"]),
	}, true
}

func bilibiliUsersFromSearchDocument(document map[string]any) []bilibiliResolvedUser {
	data, _ := document["data"].(map[string]any)
	result, _ := data["result"].([]any)
	users := make([]bilibiliResolvedUser, 0, len(result))
	for _, item := range result {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		uid := bilibiliIDText(data["mid"])
		name := cleanBilibiliUserText(firstNonEmpty(data["uname"], data["name"]))
		if !isDigits(uid) || name == "" {
			continue
		}
		users = append(users, bilibiliResolvedUser{
			UID:       uid,
			Name:      name,
			AvatarURL: cleanBilibiliUserURL(firstNonEmpty(data["upic"], data["face"], data["avatar"])),
			Fans:      intFromAny(data["fans"]),
		})
		if len(users) >= 5 {
			break
		}
	}
	return users
}

func cleanBilibiliUserText(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	text = bilibiliHTMLTagPattern.ReplaceAllString(text, "")
	return strings.TrimSpace(html.UnescapeString(text))
}

func bilibiliIDText(value any) string {
	switch typed := value.(type) {
	case json.Number:
		return typed.String()
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return cleanBilibiliUserText(value)
	}
}

func cleanBilibiliUserURL(value any) string {
	text := cleanBilibiliUserText(value)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "//") {
		return "https:" + text
	}
	if strings.Contains(text, "://") {
		return text
	}
	return ""
}

func firstNonEmpty(values ...any) any {
	for _, value := range values {
		if cleanBilibiliUserText(value) != "" {
			return value
		}
	}
	return nil
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		number, _ := typed.Int64()
		return int(number)
	case string:
		number, _ := strconv.Atoi(strings.TrimSpace(typed))
		return number
	default:
		return 0
	}
}
