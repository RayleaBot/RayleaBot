package app

import (
	"errors"
	"net/http"
)

var errSystemTaskUnavailable = errors.New("system task service unavailable")

type systemHTTPError struct {
	statusCode int
	code       string
	message    string
	messageKey string
	details    map[string]any
}

func internalSystemHTTPError() *systemHTTPError {
	return &systemHTTPError{
		statusCode: http.StatusInternalServerError,
		code:       codeInternalError,
		message:    "内部错误",
		messageKey: "errors.platform.internal_error",
	}
}

func invalidSystemHTTPError(details map[string]any) *systemHTTPError {
	return &systemHTTPError{
		statusCode: http.StatusBadRequest,
		code:       codeInvalidRequest,
		message:    "请求参数不合法",
		messageKey: "errors.platform.invalid_request",
		details:    details,
	}
}

func missingSystemResourceHTTPError(details map[string]any) *systemHTTPError {
	return &systemHTTPError{
		statusCode: http.StatusNotFound,
		code:       codeResourceMissing,
		message:    "缺少必要资源",
		messageKey: "errors.platform.resource_missing",
		details:    details,
	}
}

func writeSystemHTTPError(w http.ResponseWriter, r *http.Request, err *systemHTTPError) {
	if err == nil {
		return
	}
	writeAppError(w, r, err.statusCode, err.code, err.message, err.messageKey, err.details)
}
