package source

import (
	"net/http"
	"time"

	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/bilibili/session"
)

type SessionClient = bilibiliSession.SessionClient
type IdentityProvider = bilibiliSession.IdentityProvider
type ErrorKind = bilibiliSession.ErrorKind
type Error = bilibiliSession.Error

const (
	ErrorAuth            = bilibiliSession.ErrorAuth
	ErrorCSRF            = bilibiliSession.ErrorCSRF
	ErrorRefresh         = bilibiliSession.ErrorRefresh
	ErrorRiskControl     = bilibiliSession.ErrorRiskControl
	ErrorCaptcha         = bilibiliSession.ErrorCaptcha
	ErrorRateLimit       = bilibiliSession.ErrorRateLimit
	ErrorSignature       = bilibiliSession.ErrorSignature
	ErrorTicket          = bilibiliSession.ErrorTicket
	ErrorDevice          = bilibiliSession.ErrorDevice
	ErrorNotFound        = bilibiliSession.ErrorNotFound
	ErrorBadRequest      = bilibiliSession.ErrorBadRequest
	ErrorServer          = bilibiliSession.ErrorServer
	ErrorInvalidResponse = bilibiliSession.ErrorInvalidResponse
	ErrorUpstream        = bilibiliSession.ErrorUpstream
)

func NewIdentityProvider(now func() time.Time) *IdentityProvider {
	return bilibiliSession.NewIdentityProvider(now)
}

func NewSessionClient(transport http.RoundTripper, now func() time.Time, identity *IdentityProvider) *SessionClient {
	return bilibiliSession.NewSessionClient(transport, now, identity)
}

func cookieValues(cookie string) map[string]string {
	return bilibiliSession.CookieValues(cookie)
}

func apiError(httpStatus, code int, message string, body []byte) error {
	return bilibiliSession.APIError(httpStatus, code, message, body)
}

func classifyHTTPStatus(status int) ErrorKind {
	return bilibiliSession.ClassifyHTTPStatus(status)
}

func asBilibiliError(err error) *Error {
	return bilibiliSession.AsError(err)
}

func isBilibiliAuthError(err error) bool {
	return bilibiliSession.IsAuthError(err)
}

func isBilibiliRiskControlError(err error) bool {
	return bilibiliSession.IsRiskControlError(err)
}

func isBilibiliRiskControlErrorText(value string) bool {
	return bilibiliSession.IsRiskControlErrorText(value)
}

func shouldRetryWBI(err error) bool {
	return bilibiliSession.ShouldRetryWBI(err)
}

func isBilibiliURLForWBI(rawURL string) bool {
	return bilibiliSession.IsBilibiliURLForWBI(rawURL)
}
