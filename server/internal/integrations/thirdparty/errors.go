package thirdparty

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type ErrorKind string

const (
	ErrorAuth            ErrorKind = "auth"
	ErrorCSRF            ErrorKind = "csrf"
	ErrorRiskControl     ErrorKind = "risk_control"
	ErrorCaptcha         ErrorKind = "captcha"
	ErrorRateLimit       ErrorKind = "rate_limit"
	ErrorSignature       ErrorKind = "signature"
	ErrorNotFound        ErrorKind = "not_found"
	ErrorBadRequest      ErrorKind = "bad_request"
	ErrorServer          ErrorKind = "server"
	ErrorInvalidResponse ErrorKind = "invalid_response"
	ErrorUpstream        ErrorKind = "upstream"
	ErrorNetwork         ErrorKind = "network"
	ErrorExpired         ErrorKind = "expired"
)

type ThirdPartyError struct {
	Platform   string
	Kind       ErrorKind
	Code       int
	HTTPStatus int
	Message    string
	Body       string
	Err        error
}

func (e *ThirdPartyError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{e.Platform, string(e.Kind)}
	if e.Code != 0 {
		parts = append(parts, "code "+strconv.Itoa(e.Code))
	}
	if e.HTTPStatus != 0 {
		parts = append(parts, "HTTP "+strconv.Itoa(e.HTTPStatus))
	}
	if strings.TrimSpace(e.Message) != "" {
		parts = append(parts, strings.TrimSpace(e.Message))
	}
	if e.Err != nil && strings.TrimSpace(e.Err.Error()) != "" {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, ": ")
}

func (e *ThirdPartyError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func AsThirdPartyError(err error) *ThirdPartyError {
	var target *ThirdPartyError
	if errors.As(err, &target) {
		return target
	}
	return nil
}

func IsRiskControlError(err error) bool {
	tpErr := AsThirdPartyError(err)
	return tpErr != nil && tpErr.Kind == ErrorRiskControl
}

func IsRateLimitError(err error) bool {
	tpErr := AsThirdPartyError(err)
	return tpErr != nil && tpErr.Kind == ErrorRateLimit
}

func IsRequestCooldownError(err error) bool {
	tpErr := AsThirdPartyError(err)
	if tpErr == nil {
		return false
	}
	return tpErr.Kind == ErrorRiskControl || tpErr.Kind == ErrorRateLimit
}

func ClassifyHTTPStatus(status int) ErrorKind {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrorAuth
	case http.StatusBadRequest:
		return ErrorBadRequest
	case http.StatusNotFound:
		return ErrorNotFound
	case http.StatusTooManyRequests:
		return ErrorRateLimit
	default:
		if status >= 500 {
			return ErrorServer
		}
		if status >= 400 {
			return ErrorUpstream
		}
		return ErrorUpstream
	}
}

func NewPlatformError(platform string, kind ErrorKind, code int, httpStatus int, message string, err error) *ThirdPartyError {
	return &ThirdPartyError{
		Platform:   platform,
		Kind:       kind,
		Code:       code,
		HTTPStatus: httpStatus,
		Message:    strings.TrimSpace(message),
		Err:        err,
	}
}
