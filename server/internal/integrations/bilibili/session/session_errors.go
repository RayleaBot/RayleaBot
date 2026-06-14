package session

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
	ErrorRefresh         ErrorKind = "cookie_refresh"
	ErrorRiskControl     ErrorKind = "risk_control"
	ErrorCaptcha         ErrorKind = "captcha"
	ErrorRateLimit       ErrorKind = "rate_limit"
	ErrorSignature       ErrorKind = "signature"
	ErrorTicket          ErrorKind = "ticket"
	ErrorDevice          ErrorKind = "device"
	ErrorNotFound        ErrorKind = "not_found"
	ErrorBadRequest      ErrorKind = "bad_request"
	ErrorServer          ErrorKind = "server"
	ErrorInvalidResponse ErrorKind = "invalid_response"
	ErrorUpstream        ErrorKind = "upstream"
)

type Error struct {
	Kind       ErrorKind
	Code       int
	HTTPStatus int
	Message    string
	Body       string
	Err        error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{"bilibili", string(e.Kind)}
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

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func validateCookieForLogin(cookie string) error {
	if strings.TrimSpace(cookieValues(cookie)["SESSDATA"]) == "" {
		return &Error{Kind: ErrorAuth, Message: "SESSDATA missing"}
	}
	return nil
}

func ValidateCookieForLogin(cookie string) error {
	return validateCookieForLogin(cookie)
}

func apiError(httpStatus, code int, message string, body []byte) error {
	text := strings.TrimSpace(message)
	if text == "" {
		text = responseExcerpt(body)
	}
	kind := classifyBilibiliCode(httpStatus, code)
	if kind == ErrorRiskControl && ExtractVVoucher(body) != "" {
		kind = ErrorCaptcha
	}
	return &Error{Kind: kind, Code: code, HTTPStatus: httpStatus, Message: text, Body: string(body)}
}

func APIError(httpStatus, code int, message string, body []byte) error {
	return apiError(httpStatus, code, message, body)
}

func classifyBilibiliCode(httpStatus, code int) ErrorKind {
	switch code {
	case -101, -102, -658:
		return ErrorAuth
	case -111:
		return ErrorCSRF
	case -352, 352, -412:
		return ErrorRiskControl
	case -509, -799:
		return ErrorRateLimit
	case -404:
		return ErrorNotFound
	case -400:
		return ErrorBadRequest
	case -500, -503, -504:
		return ErrorServer
	}
	return classifyHTTPStatus(httpStatus)
}

func classifyHTTPStatus(status int) ErrorKind {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrorAuth
	case http.StatusBadRequest:
		return ErrorBadRequest
	case http.StatusNotFound:
		return ErrorNotFound
	case http.StatusPreconditionFailed, http.StatusTooManyRequests:
		if status == http.StatusTooManyRequests {
			return ErrorRateLimit
		}
		return ErrorRiskControl
	default:
		if status >= 500 {
			return ErrorServer
		}
		return ErrorUpstream
	}
}

func ClassifyHTTPStatus(status int) ErrorKind {
	return classifyHTTPStatus(status)
}

func asBilibiliError(err error) *Error {
	var target *Error
	if errors.As(err, &target) {
		return target
	}
	return nil
}

func AsError(err error) *Error {
	return asBilibiliError(err)
}

func isBilibiliAuthError(err error) bool {
	biliErr := asBilibiliError(err)
	return biliErr != nil && biliErr.Kind == ErrorAuth
}

func IsAuthError(err error) bool {
	return isBilibiliAuthError(err)
}

func isBilibiliRiskControlError(err error) bool {
	biliErr := asBilibiliError(err)
	return biliErr != nil && biliErr.Kind == ErrorRiskControl
}

func IsRiskControlError(err error) bool {
	return isBilibiliRiskControlError(err)
}

func isBilibiliRiskControlErrorText(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return false
	}
	return strings.Contains(text, "risk_control") || strings.Contains(text, "code -352")
}

func IsRiskControlErrorText(value string) bool {
	return isBilibiliRiskControlErrorText(value)
}

func shouldRetryWBI(err error) bool {
	biliErr := asBilibiliError(err)
	if biliErr == nil {
		return false
	}
	return biliErr.Kind == ErrorRiskControl || biliErr.Kind == ErrorSignature || biliErr.Code == -403 || biliErr.Code == 403
}

func ShouldRetryWBI(err error) bool {
	return shouldRetryWBI(err)
}

func isBilibiliRequestCooldownError(err error) bool {
	biliErr := asBilibiliError(err)
	if biliErr == nil {
		return false
	}
	return biliErr.Kind == ErrorRiskControl || biliErr.Kind == ErrorRateLimit
}
