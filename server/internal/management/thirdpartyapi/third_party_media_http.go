package thirdpartyapi

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

func (h *ThirdPartyHandlers) HandleThirdPartyMedia() http.HandlerFunc {
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
