package app

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
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

		items, err := a.listLogSummaries(r.Context(), logging.Query{
			Level:     levelFilter,
			Source:    sourceFilter,
			PluginID:  pluginIDFilter,
			RequestID: requestIDFilter,
			Limit:     limit,
		})
		if err != nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		writeAuthJSON(w, http.StatusOK, logListResponse{Items: items})
	}
}

func (a *App) listLogSummaries(ctx context.Context, query logging.Query) ([]logging.Summary, error) {
	if a != nil && a.LogRepository != nil {
		return a.LogRepository.ListSummaries(ctx, query)
	}

	items := make([]logging.Summary, 0)
	if a == nil || a.Logs == nil {
		return items, nil
	}
	for _, summary := range a.Logs.Snapshot() {
		if query.Level != "" && summary.Level != query.Level {
			continue
		}
		if query.Source != "" && summary.Source != query.Source {
			continue
		}
		if query.PluginID != "" && summary.PluginID != query.PluginID {
			continue
		}
		if query.RequestID != "" && summary.RequestID != query.RequestID {
			continue
		}
		items = append(items, summary)
	}
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[len(items)-query.Limit:]
	}
	return items, nil
}

func isAllowedLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}
