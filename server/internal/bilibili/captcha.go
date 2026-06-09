package bilibili

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	identity *IdentityProvider
}

// NewCaptchaClient creates a captcha client that shares the proxy and identity configuration.
func NewCaptchaClient(transport http.RoundTripper, identity *IdentityProvider) *CaptchaClient {
	return &CaptchaClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   defaultRequestTimeout,
		},
		identity: identity,
	}
}

// RegisterChallenge registers a v_voucher with the gaia-vgate service and returns geetest challenge data.
func (c *CaptchaClient) RegisterChallenge(ctx context.Context, vVoucher, cookie string) (*CaptchaChallenge, error) {
	form := url.Values{}
	form.Set("v_voucher", vVoucher)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, captchaRegisterURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	c.identity.ApplyHeaders(req, http.MethodPost)
	req.Header.Set("Cookie", cookie)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			GT        string `json:"gt"`
			Challenge string `json:"challenge"`
			Key       string `json:"key"`
			Type      string `json:"type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if doc.Code != 0 {
		return nil, &Error{Kind: ErrorCaptcha, Code: doc.Code, Message: doc.Message}
	}
	return &CaptchaChallenge{
		GT:        doc.Data.GT,
		Challenge: doc.Data.Challenge,
		Key:       doc.Data.Key,
		Type:      doc.Data.Type,
	}, nil
}

// Validate submits the solved captcha token and returns a grisk_id.
func (c *CaptchaClient) Validate(ctx context.Context, challenge *CaptchaChallenge, geetestToken, geetestValidate, geetestSeccode, csrf, cookie string) (*CaptchaResult, error) {
	form := url.Values{}
	form.Set("type", challenge.Type)
	form.Set("gt", challenge.GT)
	form.Set("challenge", challenge.Challenge)
	form.Set("token", geetestToken)
	form.Set("validate", geetestValidate)
	form.Set("seccode", geetestSeccode)
	form.Set("csrf", csrf)
	form.Set("key", challenge.Key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, captchaValidateURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	c.identity.ApplyHeaders(req, http.MethodPost)
	req.Header.Set("Cookie", cookie)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			GriskID string `json:"grisk_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if doc.Code != 0 {
		return nil, &Error{Kind: ErrorCaptcha, Code: doc.Code, Message: doc.Message}
	}
	return &CaptchaResult{
		GriskID: doc.Data.GriskID,
	}, nil
}

// TrySolve attempts to solve a captcha registered with the given v_voucher.
// It runs synchronously and returns a result only when successful.
// Failure does not affect existing monitoring behavior.
func (c *CaptchaClient) TrySolve(ctx context.Context, vVoucher, cookie string) (*CaptchaResult, error) {
	challenge, err := c.RegisterChallenge(ctx, vVoucher, cookie)
	if err != nil {
		return nil, fmt.Errorf("captcha register: %w", err)
	}
	geetestToken, geetestValidate, geetestSeccode, err := attemptGeetestBypass(ctx, challenge)
	if err != nil {
		return nil, fmt.Errorf("geetest bypass: %w", err)
	}
	csrf := cookieValues(cookie)["bili_jct"]
	result, err := c.Validate(ctx, challenge, geetestToken, geetestValidate, geetestSeccode, csrf, cookie)
	if err != nil {
		return nil, fmt.Errorf("captcha validate: %w", err)
	}
	result.VVoucher = vVoucher
	return result, nil
}

// attemptGeetestBypass tries a headless geetest bypass.
// This is a best-effort attempt; geetest v4 is difficult to bypass without ML or a third-party service.
func attemptGeetestBypass(_ context.Context, _ *CaptchaChallenge) (string, string, string, error) {
	return "", "", "", fmt.Errorf("geetest headless bypass not implemented")
}
