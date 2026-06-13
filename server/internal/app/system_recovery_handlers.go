package app

import (
	"net/http"
	"strings"
)

func (h *systemHTTPHandlers) handleSystemRecoveryRecheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.system == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		taskID, systemErr := h.system.submitRecoveryRecheckTask()
		if systemErr != nil {
			writeSystemHTTPError(w, r, systemErr)
			return
		}

		writeAuthJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}

func (h *systemHTTPHandlers) handleSystemRecoveryConfirm() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h == nil || h.system == nil {
			writeAppError(w, r, http.StatusInternalServerError, codeInternalError, "内部错误", "errors.platform.internal_error", nil)
			return
		}

		req, err := decodeRecoveryConfirmRequest(r)
		if err != nil {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}
		reviewIDs, note, ok := normalizeRecoveryConfirmRequest(req)
		if !ok {
			writeAppError(w, r, http.StatusBadRequest, codeInvalidRequest, "请求参数不合法", "errors.platform.invalid_request", nil)
			return
		}

		if systemErr := h.system.validateRecoveryConfirmRequest(reviewIDs, note); systemErr != nil {
			writeSystemHTTPError(w, r, systemErr)
			return
		}

		claims, ok := ClaimsFromContext(r.Context())
		if !ok || strings.TrimSpace(claims.Subject) == "" {
			writeAuthError(w, r, http.StatusUnauthorized, codePermissionDenied, "当前用户无权执行该操作", "errors.permission.denied")
			return
		}
		operatorID := strings.TrimSpace(claims.Subject)

		taskID, systemErr := h.system.submitRecoveryConfirmTask(reviewIDs, note, operatorID)
		if systemErr != nil {
			writeSystemHTTPError(w, r, systemErr)
			return
		}

		writeAuthJSON(w, http.StatusAccepted, taskAcceptedResponse{TaskID: taskID})
	}
}
