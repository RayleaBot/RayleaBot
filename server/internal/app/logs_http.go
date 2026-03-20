package app

import (
	"net/http"
	"strconv"
	"strings"

	"rayleabot/server/internal/logging"
)

type logListResponse struct {
	Items []logging.Summary `json:"items"`
}

func (a *App) handleLogsList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		levelFilter := strings.TrimSpace(r.URL.Query().Get("level"))
		if levelFilter != "" && !isAllowedLogLevel(levelFilter) {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		sourceFilter := strings.TrimSpace(r.URL.Query().Get("source"))
		pluginIDFilter := strings.TrimSpace(r.URL.Query().Get("plugin_id"))
		requestIDFilter := strings.TrimSpace(r.URL.Query().Get("request_id"))

		limit := 50
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > 200 {
				writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			limit = parsed
		}

		items := make([]logging.Summary, 0)
		for _, summary := range a.Logs.Snapshot() {
			if levelFilter != "" && summary.Level != levelFilter {
				continue
			}
			if sourceFilter != "" && summary.Source != sourceFilter {
				continue
			}
			if pluginIDFilter != "" && summary.PluginID != pluginIDFilter {
				continue
			}
			if requestIDFilter != "" && summary.RequestID != requestIDFilter {
				continue
			}
			items = append(items, summary)
		}
		if len(items) > limit {
			items = items[len(items)-limit:]
		}

		writeAuthJSON(w, http.StatusOK, logListResponse{Items: items})
	}
}

func isAllowedLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}
