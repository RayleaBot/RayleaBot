package bilibili

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

const (
	captchaRegisterURL = "https://api.bilibili.com/x/gaia-vgate/v1/register"
	captchaValidateURL = "https://api.bilibili.com/x/gaia-vgate/v1/validate"

	geetestJSURL = "https://static.geetest.com/static/js/fullpage.9.2.4.js"
)

var (
	geetestKeyMu  sync.Mutex
	geetestKeyVal string
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
func (c *CaptchaClient) TrySolve(ctx context.Context, vVoucher, cookie string) (*CaptchaResult, error) {
	challenge, err := c.RegisterChallenge(ctx, vVoucher, cookie)
	if err != nil {
		return nil, fmt.Errorf("captcha register: %w", err)
	}
	geetestToken, geetestValidate, geetestSeccode, err := c.attemptGeetestBypass(ctx, challenge)
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

// attemptGeetestBypass tries a headless geetest v4 bypass using static key extraction.
func (c *CaptchaClient) attemptGeetestBypass(ctx context.Context, challenge *CaptchaChallenge) (string, string, string, error) {
	key, err := fetchGeetestKey(ctx, c.client)
	if err != nil {
		return "", "", "", fmt.Errorf("geetest key fetch: %w", err)
	}
	return computeGeetestResponse(challenge, key)
}

// fetchGeetestKey fetches the geetest JS file and extracts the static key, with caching.
func fetchGeetestKey(ctx context.Context, client *http.Client) (string, error) {
	geetestKeyMu.Lock()
	cached := geetestKeyVal
	geetestKeyMu.Unlock()
	if cached != "" {
		return cached, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geetestJSURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	js, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	key := extractGeetestKey(string(js))
	if key == "" {
		return "", fmt.Errorf("geetest static key not found in JS")
	}

	geetestKeyMu.Lock()
	geetestKeyVal = key
	geetestKeyMu.Unlock()
	return key, nil
}

var geetestKeyPattern = regexp.MustCompile(`"static_key"\s*:\s*"([a-fA-F0-9]+)"`)

func extractGeetestKey(js string) string {
	match := geetestKeyPattern.FindStringSubmatch(js)
	if len(match) >= 2 {
		return strings.ToLower(match[1])
	}
	return ""
}

func computeGeetestResponse(challenge *CaptchaChallenge, key string) (token, validate, seccode string, err error) {
	raw := challenge.Challenge + key
	hash := md5.Sum([]byte(raw))
	validate = hex.EncodeToString(hash[:])[:16]
	seccode = validate + "|jordan"

	tokenRaw := challenge.Challenge + key + challenge.GT
	tokenHash := md5.Sum([]byte(tokenRaw))
	token = hex.EncodeToString(tokenHash[:])

	return token, validate, seccode, nil
}
