package management

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/RayleaBot/RayleaBot/server/internal/httpapi"
)

const maxRecoveryConfirmNoteRunes = 500

type recoveryConfirmRequest struct {
	ReviewIDs []string `json:"review_ids"`
	Note      string   `json:"note,omitempty"`
}

func (h *SystemHandlers) HandleSystemRecoveryRecheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.system == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, systemCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		taskID, systemErr := h.system.SubmitRecoveryRecheckTask()
		if systemErr != nil {
			WriteSystemError(w, r, systemErr)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func (h *SystemHandlers) HandleSystemRecoveryConfirm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.system == nil {
			httpapi.WriteError(w, r, http.StatusInternalServerError, systemCodeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		req, err := decodeRecoveryConfirmRequest(r)
		if err != nil {
			httpapi.WriteError(w, r, http.StatusBadRequest, systemCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		reviewIDs, note, ok := normalizeRecoveryConfirmRequest(req)
		if !ok {
			httpapi.WriteError(w, r, http.StatusBadRequest, systemCodeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		if systemErr := h.system.ValidateRecoveryConfirmRequest(reviewIDs, note); systemErr != nil {
			WriteSystemError(w, r, systemErr)
			return
		}

		claims, ok := ClaimsFromContext(r.Context())
		if !ok || strings.TrimSpace(claims.Subject) == "" {
			httpapi.WriteError(w, r, http.StatusUnauthorized, systemCodePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied", nil)
			return
		}
		operatorID := strings.TrimSpace(claims.Subject)

		taskID, systemErr := h.system.SubmitRecoveryConfirmTask(reviewIDs, note, operatorID)
		if systemErr != nil {
			WriteSystemError(w, r, systemErr)
			return
		}

		httpapi.WriteJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func decodeRecoveryConfirmRequest(r *http.Request) (recoveryConfirmRequest, error) {
	if r == nil || r.Body == nil {
		return recoveryConfirmRequest{}, io.EOF
	}
	var req recoveryConfirmRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return recoveryConfirmRequest{}, err
		}
		return recoveryConfirmRequest{}, err
	}
	return req, nil
}

func normalizeRecoveryConfirmRequest(req recoveryConfirmRequest) ([]string, string, bool) {
	reviewIDs := make([]string, 0, len(req.ReviewIDs))
	seen := map[string]struct{}{}
	for _, reviewID := range req.ReviewIDs {
		reviewID = strings.TrimSpace(reviewID)
		if reviewID == "" {
			return nil, "", false
		}
		if _, ok := seen[reviewID]; ok {
			continue
		}
		seen[reviewID] = struct{}{}
		reviewIDs = append(reviewIDs, reviewID)
	}
	if len(reviewIDs) == 0 {
		return nil, "", false
	}
	note := strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > maxRecoveryConfirmNoteRunes {
		return nil, "", false
	}
	return reviewIDs, note, true
}
