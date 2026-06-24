package thirdpartyapi

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	thirdpartymedia "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/media"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/common"
	"github.com/RayleaBot/RayleaBot/server/internal/integrations/thirdparty"
)

const (
	thirdPartyUserResolveTimeout       = 24 * time.Second
	thirdPartyResolvedAvatarTimeout    = 3 * time.Second
	thirdPartyResolvedAvatarMaxPayload = 256 << 10
)

type thirdPartyResolvedUser struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

type thirdPartyUserResolveResponse struct {
	Platform   string                   `json:"platform"`
	Query      string                   `json:"query"`
	Exact      bool                     `json:"exact"`
	User       *thirdPartyResolvedUser  `json:"user,omitempty"`
	Candidates []thirdPartyResolvedUser `json:"candidates"`
	Message    string                   `json:"message,omitempty"`
}

type thirdPartyAccountCookieReader interface {
	ListEnabled(context.Context, string) ([]thirdparty.Account, error)
	ReadCookie(context.Context, thirdparty.Account) (string, error)
}

func (h *ThirdPartyHandlers) HandleThirdPartyUserResolve() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		platform, err := thirdparty.NormalizePlatform(r.URL.Query().Get("platform"))
		if err != nil || platform == thirdparty.PlatformBilibili {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "三方平台不正确", "errors.platform.invalid_request", nil)
			return
		}
		query := strings.TrimSpace(r.URL.Query().Get("query"))
		if query == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, "platform.invalid_request", "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), thirdPartyUserResolveTimeout)
		defer cancel()
		response, err := h.resolveThirdPartyUser(ctx, platform, query)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadGateway, "platform.upstream_request_failed", "三方平台用户信息读取失败", "errors.platform.upstream_request_failed", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, response)
	}
}

func (h *ThirdPartyHandlers) resolveThirdPartyUser(ctx context.Context, platform, query string) (thirdPartyUserResolveResponse, error) {
	response := thirdPartyUserResolveResponse{
		Platform:   platform,
		Query:      query,
		Candidates: []thirdPartyResolvedUser{},
	}
	profiles, exact, err := h.resolveThirdPartyProfiles(ctx, platform, query)
	if err != nil {
		return response, err
	}
	response.Candidates = h.thirdPartyResolvedUsersFromProfiles(ctx, platform, profiles)
	if len(response.Candidates) == 0 {
		response.Message = thirdPartyResolveNotFoundMessage(platform)
		return response, nil
	}
	if exact {
		response.Exact = true
		user := pickThirdPartyResolvedUser(response.Candidates, query)
		response.User = &user
	}
	return response, nil
}

func (h *ThirdPartyHandlers) resolveThirdPartyProfiles(ctx context.Context, platform, query string) ([]thirdparty.AccountProfile, bool, error) {
	if h == nil || h.userResolver == nil {
		return nil, false, nil
	}
	return h.userResolver.ResolveProfiles(ctx, platform, query, h.platformCookieMaps(ctx, platform))
}

func (h *ThirdPartyHandlers) platformCookieMaps(ctx context.Context, platform string) []map[string]string {
	if h == nil || h.accounts == nil {
		return nil
	}
	reader, ok := h.accounts.(thirdPartyAccountCookieReader)
	if !ok {
		return nil
	}
	accounts, err := reader.ListEnabled(ctx, platform)
	if err != nil {
		return nil
	}
	cookieMaps := make([]map[string]string, 0, len(accounts))
	for _, account := range accounts {
		cookie, err := reader.ReadCookie(ctx, account)
		if err != nil {
			continue
		}
		cookies := common.CookieMapFromHeader(cookie)
		if len(cookies) > 0 {
			cookieMaps = append(cookieMaps, cookies)
		}
	}
	return cookieMaps
}

func (h *ThirdPartyHandlers) thirdPartyResolvedUsersFromProfiles(ctx context.Context, platform string, profiles []thirdparty.AccountProfile) []thirdPartyResolvedUser {
	items := make([]thirdPartyResolvedUser, 0, len(profiles))
	for _, profile := range profiles {
		uid := strings.TrimSpace(profile.UID)
		name := strings.TrimSpace(profile.Nickname)
		if uid == "" || name == "" {
			continue
		}
		items = append(items, thirdPartyResolvedUser{
			UID:       uid,
			Name:      name,
			AvatarURL: h.thirdPartyResolvedAvatarURL(ctx, platform, profile.AvatarURL),
		})
	}
	return items
}

func (h *ThirdPartyHandlers) thirdPartyResolvedAvatarURL(ctx context.Context, platform string, value string) string {
	avatarURL := strings.TrimSpace(value)
	if avatarURL == "" || platform != thirdparty.PlatformWeibo {
		return avatarURL
	}
	fetchCtx, cancel := context.WithTimeout(ctx, thirdPartyResolvedAvatarTimeout)
	defer cancel()
	resource, err := thirdpartymedia.Fetch(fetchCtx, h.mediaClient, avatarURL)
	if err != nil || len(resource.Body) == 0 || len(resource.Body) > thirdPartyResolvedAvatarMaxPayload {
		return avatarURL
	}
	return "data:" + resource.ContentType + ";base64," + base64.StdEncoding.EncodeToString(resource.Body)
}

func pickThirdPartyResolvedUser(candidates []thirdPartyResolvedUser, query string) thirdPartyResolvedUser {
	normalized := strings.TrimSpace(strings.ToLower(strings.TrimPrefix(query, "@")))
	for _, candidate := range candidates {
		if strings.ToLower(strings.TrimSpace(candidate.UID)) == normalized || strings.ToLower(strings.TrimSpace(candidate.Name)) == normalized {
			return candidate
		}
	}
	return candidates[0]
}

func thirdPartyResolveNotFoundMessage(platform string) string {
	switch platform {
	case thirdparty.PlatformWeibo:
		return "没有找到这个微博用户。"
	case thirdparty.PlatformDouyin:
		return "没有找到这个抖音用户。"
	case thirdparty.PlatformNeteaseMusic:
		return "没有找到这个网易云音乐对象。"
	default:
		return "没有找到这个三方平台对象。"
	}
}
