package bilibili

import (
	"net/http"
	"time"

	bilibiliCaptcha "github.com/RayleaBot/RayleaBot/server/internal/bilibili/captcha"
	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/bilibili/session"
)

type IdentityProvider = bilibiliSession.IdentityProvider
type AccountClient = bilibiliSession.AccountClient
type PreparedCookie = bilibiliSession.PreparedCookie
type SessionClient = bilibiliSession.SessionClient
type QRLoginService = bilibiliSession.QRLoginService
type QRLoginCreateResult = bilibiliSession.QRLoginCreateResult
type QRLoginPollResult = bilibiliSession.QRLoginPollResult
type ErrorKind = bilibiliSession.ErrorKind
type Error = bilibiliSession.Error
type CaptchaChallenge = bilibiliCaptcha.CaptchaChallenge
type CaptchaResult = bilibiliCaptcha.CaptchaResult
type CaptchaClient = bilibiliCaptcha.CaptchaClient

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

	QRLoginPendingScan    = bilibiliSession.QRLoginPendingScan
	QRLoginPendingConfirm = bilibiliSession.QRLoginPendingConfirm
	QRLoginExpired        = bilibiliSession.QRLoginExpired
	QRLoginSucceeded      = bilibiliSession.QRLoginSucceeded
)

func NewIdentityProvider(now func() time.Time) *IdentityProvider {
	return bilibiliSession.NewIdentityProvider(now)
}

func NewAccountClient(transport http.RoundTripper, now func() time.Time, identity *IdentityProvider) *AccountClient {
	return bilibiliSession.NewAccountClient(transport, now, identity)
}

func NewSessionClient(transport http.RoundTripper, now func() time.Time, identity *IdentityProvider) *SessionClient {
	return bilibiliSession.NewSessionClient(transport, now, identity)
}

func NewQRLoginService(transport http.RoundTripper, now func() time.Time) *QRLoginService {
	return bilibiliSession.NewQRLoginService(transport, now)
}

func NewCaptchaClient(transport http.RoundTripper, identity *IdentityProvider) *CaptchaClient {
	return bilibiliCaptcha.NewCaptchaClient(transport, identity)
}

func ExtractVVoucher(body []byte) string {
	return bilibiliCaptcha.ExtractVVoucher(body)
}

func validateCookieForLogin(cookie string) error {
	return bilibiliSession.ValidateCookieForLogin(cookie)
}

func cookieValues(cookie string) map[string]string {
	return bilibiliSession.CookieValues(cookie)
}

func mergeCookieValues(cookie string, updates map[string]string) string {
	return bilibiliSession.MergeCookieValues(cookie, updates)
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
