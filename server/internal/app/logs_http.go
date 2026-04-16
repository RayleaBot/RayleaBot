package app

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		levelFilter := strings.TrimSpace(r.URL.Query().Get("level"))
		if levelFilter != "" && !isAllowedLogLevel(levelFilter) {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		sourceFilter := strings.TrimSpace(r.URL.Query().Get("source"))
		protocolFilter := strings.TrimSpace(r.URL.Query().Get("protocol"))
		if protocolFilter != "" && !logging.IsSupportedProtocol(protocolFilter) {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		pluginIDFilter := strings.TrimSpace(r.URL.Query().Get("plugin_id"))
		requestIDFilter := strings.TrimSpace(r.URL.Query().Get("request_id"))
		cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
		direction := logging.PageDirection(strings.TrimSpace(r.URL.Query().Get("direction")))
		if direction != "" && direction != logging.PageDirectionOlder && direction != logging.PageDirectionNewer {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		limit := 50
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > maxLogPageLimit {
				writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
				return
			}
			limit = parsed
		}

		scope, err := parseLogScope(r.URL.Query().Get("scope"))
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		startAt, endAt, err := parseLogTimeRange(scope, r.URL.Query().Get("start_at"), r.URL.Query().Get("end_at"))
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		pageQuery := logging.PageQuery{
			Level:     levelFilter,
			Source:    sourceFilter,
			Protocol:  protocolFilter,
			PluginID:  pluginIDFilter,
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

func (s *logService) listLogSummaries(ctx context.Context, query logging.Query) ([]logging.Summary, error) {
	if s != nil && s.repository != nil {
		return s.repository.ListSummaries(ctx, query)
	}

	items := make([]logging.Summary, 0)
	if s == nil || s.stream == nil {
		return items, nil
	}
	for _, summary := range s.stream.Snapshot() {
		if query.BootID != "" && summary.BootID != query.BootID {
			continue
		}
		if query.Level != "" && summary.Level != query.Level {
			continue
		}
		if query.Source != "" && summary.Source != query.Source {
			continue
		}
		if query.Protocol != "" && summary.Protocol != query.Protocol {
			continue
		}
		if query.PluginID != "" && summary.PluginID != query.PluginID {
			continue
		}
		if query.RequestID != "" && summary.RequestID != query.RequestID {
			continue
		}
		if !logSummaryMatchesTimeRange(summary, query.StartAt, query.EndAt) {
			continue
		}
		items = append(items, summary)
	}
	if query.Limit > 0 && len(items) > query.Limit {
		items = items[len(items)-query.Limit:]
	}
	return items, nil
}

func parseLogScope(raw string) (logScope, error) {
	switch strings.TrimSpace(raw) {
	case "", string(logScopeHistory):
		return logScopeHistory, nil
	case string(logScopeCurrentSession):
		return logScopeCurrentSession, nil
	default:
		return "", errors.New("unsupported log scope")
	}
}

func parseLogTimeRange(scope logScope, rawStartAt, rawEndAt string) (string, string, error) {
	startAt := strings.TrimSpace(rawStartAt)
	endAt := strings.TrimSpace(rawEndAt)
	if scope == logScopeCurrentSession {
		if startAt != "" || endAt != "" {
			return "", "", errors.New("current session scope does not support time range")
		}
		return "", "", nil
	}

	startUTC, err := normalizeLogQueryTime(startAt)
	if err != nil {
		return "", "", err
	}
	endUTC, err := normalizeLogQueryTime(endAt)
	if err != nil {
		return "", "", err
	}
	if startUTC != "" && endUTC != "" && startUTC > endUTC {
		return "", "", errors.New("start_at is later than end_at")
	}
	return startUTC, endUTC, nil
}

func normalizeLogQueryTime(raw string) (string, error) {
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

func logSummaryMatchesTimeRange(summary logging.Summary, startAt, endAt string) bool {
	if startAt == "" && endAt == "" {
		return true
	}

	timestamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(summary.Timestamp))
	if err != nil {
		return false
	}
	normalized := timestamp.UTC()
	if startAt != "" {
		start, err := time.Parse(time.RFC3339, startAt)
		if err != nil || normalized.Before(start.UTC()) {
			return false
		}
	}
	if endAt != "" {
		end, err := time.Parse(time.RFC3339, endAt)
		if err != nil || normalized.After(end.UTC()) {
			return false
		}
	}
	return true
}

func (s *logService) listLogPage(ctx context.Context, query logging.PageQuery) (logging.PageResult, error) {
	if s != nil && s.repository != nil {
		return s.repository.ListPage(ctx, query)
	}

	items, err := s.listLogSummaries(ctx, logging.Query{
		Level:     query.Level,
		Source:    query.Source,
		Protocol:  query.Protocol,
		PluginID:  query.PluginID,
		RequestID: query.RequestID,
		Limit:     query.Limit,
	})
	if err != nil {
		return logging.PageResult{}, err
	}
	reversed := make([]logging.Summary, 0, len(items))
	for index := len(items) - 1; index >= 0; index-- {
		reversed = append(reversed, items[index])
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	return logging.PageResult{
		Items: reversed,
		Page: logging.PageInfo{
			Limit: limit,
		},
	}, nil
}

func (s *logService) getLogSummary(ctx context.Context, logID string) (logging.Summary, error) {
	trimmedLogID := strings.TrimSpace(logID)
	if s != nil && s.repository != nil {
		item, err := s.repository.GetSummary(ctx, trimmedLogID)
		if err == nil {
			return item, nil
		}
		if err != logging.ErrLogNotFound {
			return logging.Summary{}, err
		}
		if item, ok := s.findStreamLogSummary(trimmedLogID); ok {
			return item, nil
		}
		return logging.Summary{}, logging.ErrLogNotFound
	}

	if item, ok := s.findStreamLogSummary(trimmedLogID); ok {
		return item, nil
	}

	if s == nil || s.stream == nil {
		return logging.Summary{}, logging.ErrLogNotFound
	}

	return logging.Summary{}, logging.ErrLogNotFound
}

func (s *logService) findStreamLogSummary(logID string) (logging.Summary, bool) {
	if s == nil || s.stream == nil || logID == "" {
		return logging.Summary{}, false
	}

	for _, item := range s.stream.Snapshot() {
		if item.LogID == logID {
			return item, true
		}
	}

	return logging.Summary{}, false
}

func isAllowedLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}
