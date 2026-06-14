package logapi

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
	codeInvalidRequest  = "platform.invalid_request"
	codeResourceMissing = "platform.resource_missing"
	codeInternalError   = "platform.internal_error"
	maxPageLimit        = 200
)

type Service interface {
	CurrentBootID() string
	ListLogPage(context.Context, logging.PageQuery) (logging.PageResult, error)
	GetLogSummary(context.Context, string) (logging.Summary, error)
}

type Handlers struct {
	logs Service
}

func NewHandlers(logs Service) *Handlers {
	return &Handlers{logs: logs}
}

type listResponse struct {
	Items []logging.Summary `json:"items"`
	Page  logging.PageInfo  `json:"page"`
}

type detailResponse struct {
	logging.Summary
	Details map[string]any `json:"details"`
}

type scope string

const (
	scopeHistory        scope = "history"
	scopeCurrentSession scope = "current_session"
)

func (h *Handlers) HandleLogsList() http.HandlerFunc {
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
			if err != nil || parsed < 1 || parsed > maxPageLimit {
				writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			limit = parsed
		}

		scopeValue, err := parseScope(queryValues.Get("scope"))
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		startAt, endAt, err := parseTimeRange(scopeValue, queryValues.Get("start_at"), queryValues.Get("end_at"))
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
		if scopeValue == scopeCurrentSession {
			pageQuery.BootID = h.logs.CurrentBootID()
		}

		result, err := h.logs.ListLogPage(r.Context(), pageQuery)
		if err != nil {
			if errors.Is(err, logging.ErrInvalidCursor) {
				writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}
		writeAuthJSON(w, http.StatusOK, listResponse{
			Items: result.Items,
			Page:  result.Page,
		})
	}
}

func (h *Handlers) HandleLogDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logID := strings.TrimSpace(chi.URLParam(r, "log_id"))
		if logID == "" {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		item, err := h.logs.GetLogSummary(r.Context(), logID)
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

		writeAuthJSON(w, http.StatusOK, detailResponse{
			Summary: item,
			Details: item.Details,
		})
	}
}

func parseScope(raw string) (scope, error) {
	switch strings.TrimSpace(raw) {
	case "", string(scopeHistory):
		return scopeHistory, nil
	case string(scopeCurrentSession):
		return scopeCurrentSession, nil
	default:
		return "", errors.New("unsupported log scope")
	}
}

func parseTimeRange(scopeValue scope, rawStartAt, rawEndAt string) (string, string, error) {
	startAt := strings.TrimSpace(rawStartAt)
	endAt := strings.TrimSpace(rawEndAt)
	if scopeValue == scopeCurrentSession {
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

func writeAppError(w http.ResponseWriter, r *http.Request, statusCode int, code, message, messageKey string, details map[string]any) {
	httpapi.WriteError(w, r, statusCode, code, message, messageKey, details)
}

func writeAuthJSON(w http.ResponseWriter, statusCode int, body any) {
	httpapi.WriteJSON(w, statusCode, body)
}
