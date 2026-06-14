package captcha

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/bilibili/session"
)

const (
	captchaRegisterURL = "https://api.bilibili.com/x/gaia-vgate/v1/register"
	captchaValidateURL = "https://api.bilibili.com/x/gaia-vgate/v1/validate"
)

// ExtractVVoucher extracts the v_voucher value from a -352 risk control response body.
func ExtractVVoucher(body []byte) string {
	var doc struct {
		Data struct {
			VVoucher string `json:"v_voucher"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Data.VVoucher)
}

// CaptchaChallenge holds the geetest challenge data returned after registering a v_voucher.
type CaptchaChallenge struct {
	GT        string `json:"gt"`
	Challenge string `json:"challenge"`
	Key       string `json:"key"`
	Type      string `json:"type"`
}

// CaptchaResult holds the result of a captcha solve attempt.
type CaptchaResult struct {
	GriskID  string
	VVoucher string
}

// CaptchaClient registers v_voucher challenges and attempts headless geetest bypass.
type CaptchaClient struct {
	client   *http.Client
	identity Identity
}

type Identity interface {
	ApplyHeaders(req *http.Request, method string)
}

// NewCaptchaClient creates a captcha client that shares the proxy and identity configuration.
func NewCaptchaClient(transport http.RoundTripper, identity Identity) *CaptchaClient {
	return &CaptchaClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   bilibiliSession.DefaultRequestTimeout,
		},
		identity: identity,
	}
}

// TrySolve attempts to solve a captcha registered with the given v_voucher.
func (c *CaptchaClient) TrySolve(ctx context.Context, vVoucher, cookie string) (*CaptchaResult, error) {
	challenge, err := c.RegisterChallenge(ctx, vVoucher, cookie)
	if err != nil {
		return nil, fmt.Errorf("captcha register: %w", err)
	}
	geetestToken, geetestValidate, geetestSeccode, err := c.attemptGeetestBypass(ctx, challenge)
	if err != nil {
		return nil, fmt.Errorf("geetest bypass: %w", err)
	}
	csrf := bilibiliSession.CookieValues(cookie)["bili_jct"]
	result, err := c.Validate(ctx, challenge, geetestToken, geetestValidate, geetestSeccode, csrf, cookie)
	if err != nil {
		return nil, fmt.Errorf("captcha validate: %w", err)
	}
	result.VVoucher = vVoucher
	return result, nil
}
