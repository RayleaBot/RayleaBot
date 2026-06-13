package app

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

type logListResponse struct {
	Items []logging.Summary `json:"items"`
	Page  logging.PageInfo  `json:"page"`
}

type logDetailResponse struct {
	logging.Summary
	Details map[string]any `json:"details"`
}

const maxLogPageLimit = 200

type logScope string

const (
	logScopeHistory        logScope = "history"
	logScopeCurrentSession logScope = "current_session"
)

func (h *logHTTPHandlers) handleLogsList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryValues := r.URL.Query()
		levelFilters := normalizeRepeatedQueryValues(queryValues["level"])
		for _, levelFilter := range levelFilters {
			if !isAllowedLogLevel(levelFilter) {
				writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
		}

		sourceFilter := strings.TrimSpace(queryValues.Get("source"))
		protocolFilter := strings.TrimSpace(queryValues.Get("protocol"))
		if protocolFilter != "" && !logging.IsSupportedProtocol(protocolFilter) {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		pluginIDFilters := normalizeRepeatedQueryValues(queryValues["plugin_id"])
		requestIDFilter := strings.TrimSpace(queryValues.Get("request_id"))
		cursor := strings.TrimSpace(queryValues.Get("cursor"))
		direction := logging.PageDirection(strings.TrimSpace(queryValues.Get("direction")))
		if direction != "" && direction != logging.PageDirectionOlder && direction != logging.PageDirectionNewer {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		limit := 50
		if raw := strings.TrimSpace(queryValues.Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > maxLogPageLimit {
				writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			limit = parsed
		}

		scope, err := parseLogScope(queryValues.Get("scope"))
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		startAt, endAt, err := parseLogTimeRange(scope, queryValues.Get("start_at"), queryValues.Get("end_at"))
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		pageQuery := logging.PageQuery{
			Levels:    levelFilters,
			Source:    sourceFilter,
			Protocol:  protocolFilter,
			PluginIDs: pluginIDFilters,
			RequestID: requestIDFilter,
			StartAt:   startAt,
			EndAt:     endAt,
			Limit:     limit,
			Cursor:    cursor,
			Direction: direction,
		}
		if scope == logScopeCurrentSession {
			pageQuery.BootID = h.logs.currentBootID()
		}

		result, err := h.logs.listLogPage(r.Context(), pageQuery)
		if err != nil {
			if errors.Is(err, logging.ErrInvalidCursor) {
				writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		writeAuthJSON(w, http.StatusOK, logListResponse{
			Items: result.Items,
			Page:  result.Page,
		})
	}
}

func (h *logHTTPHandlers) handleLogDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logID := strings.TrimSpace(chi.URLParam(r, "log_id"))
		if logID == "" {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		item, err := h.logs.getLogSummary(r.Context(), logID)
		if err != nil {
			if err == logging.ErrLogNotFound {
				writeAppError(w, r, http.StatusNotFound, codeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
					"resource_type": "log",
					"log_id":        logID,
				})
				return
			}
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		writeAuthJSON(w, http.StatusOK, logDetailResponse{
			Summary: item,
			Details: item.Details,
		})
	}
}
