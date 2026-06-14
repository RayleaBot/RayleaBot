package managementhttp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func (h *BilibiliHandlers) HandleBilibiliUserResolve() http.HandlerFunc {
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

const (
	bilibiliUserInfoURL   = "https://api.bilibili.com/x/space/acc/info?mid=%s&jsonp=jsonp"
	bilibiliUserSearchURL = "https://api.bilibili.com/x/web-interface/search/type"
)

func (h *BilibiliHandlers) resolveBilibiliUser(ctx context.Context, query string) (bilibiliUserResolveResponse, error) {
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
