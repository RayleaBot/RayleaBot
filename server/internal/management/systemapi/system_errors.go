package systemapi

import (
	"net/http"

	systemmodel "github.com/RayleaBot/RayleaBot/server/internal/system/model"
)

type SystemHTTPError struct {
	statusCode int
	code       string
	message    string
	messageKey string
	details    map[string]any
}

func InternalSystemHTTPError() *SystemHTTPError {
	return &SystemHTTPError{
		statusCode: http.StatusInternalServerError,
		code:       codeInternalError,
		message:    "内部错误",
		messageKey: "errors.platform.internal_error",
	}
}

func InvalidSystemHTTPError(details map[string]any) *SystemHTTPError {
	return &SystemHTTPError{
		statusCode: http.StatusBadRequest,
		code:       codeInvalidRequest,
		message:    "请求参数不合法",
		messageKey: "errors.platform.invalid_request",
		details:    details,
	}
}

func MissingSystemResourceHTTPError(details map[string]any) *SystemHTTPError {
	return &SystemHTTPError{
		statusCode: http.StatusNotFound,
		code:       codeResourceMissing,
		message:    "缺少必要资源",
		messageKey: "errors.platform.resource_missing",
		details:    details,
	}
}

func WriteSystemHTTPError(w http.ResponseWriter, r *http.Request, err *SystemHTTPError) {
	if err == nil {
		return
	}
	writeAppError(w, r, err.statusCode, err.code, err.message, err.messageKey, err.details)
}

func WriteSystemError(w http.ResponseWriter, r *http.Request, err *systemmodel.Error) {
	WriteSystemHTTPError(w, r, systemHTTPErrorFromError(err))
}

func systemHTTPErrorFromError(err *systemmodel.Error) *SystemHTTPError {
	if err == nil {
		return nil
	}
	switch err.Reason {
	case systemmodel.ErrorReasonInvalidRequest:
		return InvalidSystemHTTPError(err.Details)
	case systemmodel.ErrorReasonResourceMissing:
		return MissingSystemResourceHTTPError(err.Details)
	default:
		return InternalSystemHTTPError()
	}
}
