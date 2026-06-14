package captcha

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	bilibiliSession "github.com/RayleaBot/RayleaBot/server/internal/integrations/bilibili/session"
)

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
		return nil, &bilibiliSession.Error{Kind: bilibiliSession.ErrorCaptcha, Code: doc.Code, Message: doc.Message}
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
		return nil, &bilibiliSession.Error{Kind: bilibiliSession.ErrorCaptcha, Code: doc.Code, Message: doc.Message}
	}
	return &CaptchaResult{
		GriskID: doc.Data.GriskID,
	}, nil
}
