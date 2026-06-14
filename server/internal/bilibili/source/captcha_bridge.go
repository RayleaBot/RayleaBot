package source

import (
	"net/http"

	bilibiliCaptcha "github.com/RayleaBot/RayleaBot/server/internal/bilibili/captcha"
)

type CaptchaChallenge = bilibiliCaptcha.CaptchaChallenge
type CaptchaResult = bilibiliCaptcha.CaptchaResult
type CaptchaClient = bilibiliCaptcha.CaptchaClient

func NewCaptchaClient(transport http.RoundTripper, identity *IdentityProvider) *CaptchaClient {
	return bilibiliCaptcha.NewCaptchaClient(transport, identity)
}

func ExtractVVoucher(body []byte) string {
	return bilibiliCaptcha.ExtractVVoucher(body)
}
