package management

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
	"github.com/RayleaBot/RayleaBot/server/internal/logging"
)

const (
	logCodeInvalidRequest  = "platform.invalid_request"
	logCodeResourceMissing = "platform.resource_missing"
	logCodeInternalError   = "platform.internal_error"
	logMaxPageLimit        = 200
)

type LogService interface {
	CurrentBootID() string
	ListLogPage(context.Context, logging.PageQuery) (logging.PageResult, error)
	GetLogSummary(context.Context, string) (logging.Summary, error)
}

type LogHandlers struct {
	logs LogService
}

func NewLogHandlers(logs LogService) *LogHandlers {
	return &LogHandlers{logs: logs}
}

func (h *LogHandlers) RegisterProtectedRoutes(router chi.Router) {
	router.Get("/api/logs", h.HandleLogsList())
	router.Get("/api/logs/{log_id}", h.HandleLogDetail())
}

type logListResponse struct {
	Items []logging.Summary `json:"items"`
	Page  logging.PageInfo  `json:"page"`
}

type logDetailResponse struct {
	logging.Summary
	Details map[string]any `json:"details"`
}

type logScope string

const (
	logScopeHistory        logScope = "history"
	logScopeCurrentSession logScope = "current_session"
)

func (h *LogHandlers) HandleLogsList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		queryValues := r.URL.Query()
		levelFilters := normalizeRepeatedQueryValues(queryValues["level"])
		for _, levelFilter := range levelFilters {
			if !isAllowedLogLevel(levelFilter) {
				httpapi.WriteError(w, r, http.StatusBadRequest, logCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
		}

		sourceFilter := strings.TrimSpace(queryValues.Get("source"))
		protocolFilter := strings.TrimSpace(queryValues.Get("protocol"))
		if protocolFilter != "" && !logging.IsSupportedProtocol(protocolFilter) {
			httpapi.WriteError(w, r, http.StatusBadRequest, logCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		pluginIDFilters := normalizeRepeatedQueryValues(queryValues["plugin_id"])
		requestIDFilter := strings.TrimSpace(queryValues.Get("request_id"))
		cursor := strings.TrimSpace(queryValues.Get("cursor"))
		direction := logging.PageDirection(strings.TrimSpace(queryValues.Get("direction")))
		if direction != "" && direction != logging.PageDirectionOlder && direction != logging.PageDirectionNewer {
			httpapi.WriteError(w, r, http.StatusBadRequest, logCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		limit := 50
		if raw := strings.TrimSpace(queryValues.Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > logMaxPageLimit {
				httpapi.WriteError(w, r, http.StatusBadRequest, logCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			limit = parsed
		}

		scopeValue, err := parseScope(queryValues.Get("scope"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, logCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		startAt, endAt, err := parseTimeRange(scopeValue, queryValues.Get("start_at"), queryValues.Get("end_at"))
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, logCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
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
		if scopeValue == logScopeCurrentSession {
			pageQuery.BootID = h.logs.CurrentBootID()
		}

		result, err := h.logs.ListLogPage(r.Context(), pageQuery)
		if err != nil {
			if errors.Is(err, logging.ErrInvalidCursor) {
				httpapi.WriteError(w, r, http.StatusBadRequest, logCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, logCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		httpapi.WriteJSON(w, http.StatusOK, logListResponse{
			Items: result.Items,
			Page:  result.Page,
		})
	}
}

func (h *LogHandlers) HandleLogDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logID := strings.TrimSpace(chi.URLParam(r, "log_id"))
		if logID == "" {
			httpapi.WriteError(w, r, http.StatusBadRequest, logCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		item, err := h.logs.GetLogSummary(r.Context(), logID)
		if err != nil {
			if err == logging.ErrLogNotFound {
				httpapi.WriteError(w, r, http.StatusNotFound, logCodeResourceMissing, "缺少必要资源", "errors.platform.resource_missing", map[string]any{
					"resource_type": "log",
					"log_id":        logID,
				})
				return
			}
			httpapi.WriteError(w, r, http.StatusInternalServerError, logCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, logDetailResponse{
			Summary: item,
			Details: item.Details,
		})
	}
}

func parseScope(raw string) (logScope, error) {
	switch strings.TrimSpace(raw) {
	case "", string(logScopeHistory):
		return logScopeHistory, nil
	case string(logScopeCurrentSession):
		return logScopeCurrentSession, nil
	default:
		return "", errors.New("unsupported log scope")
	}
}

func parseTimeRange(scopeValue logScope, rawStartAt, rawEndAt string) (string, string, error) {
	startAt := strings.TrimSpace(rawStartAt)
	endAt := strings.TrimSpace(rawEndAt)
	if scopeValue == logScopeCurrentSession {
		if startAt != "" || endAt != "" {
			return "", "", errors.New("current session scope does not support time range")
		}
		return "", "", nil
	}

	startUTC, err := normalizeQueryTime(startAt)
	if err != nil {
		return "", "", err
	}
	endUTC, err := normalizeQueryTime(endAt)
	if err != nil {
		return "", "", err
	}
	if startUTC != "" && endUTC != "" && startUTC > endUTC {
		return "", "", errors.New("start_at is later than end_at")
	}
	return startUTC, endUTC, nil
}

func normalizeQueryTime(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return "", err
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func isAllowedLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func normalizeRepeatedQueryValues(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	return normalized
}
