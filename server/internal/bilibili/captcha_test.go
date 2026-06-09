package bilibili

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestExtractVVoucherFromBody(t *testing.T) {
	t.Parallel()
	body := []byte(`{"code":-352,"message":"风控校验失败","data":{"v_voucher":"test_voucher_abc123","url":""}}`)
	got := ExtractVVoucher(body)
	if got != "test_voucher_abc123" {
		t.Fatalf("ExtractVVoucher = %q, want test_voucher_abc123", got)
	}
}

func TestExtractVVoucherEmptyBody(t *testing.T) {
	t.Parallel()
	if got := ExtractVVoucher([]byte{}); got != "" {
		t.Fatalf("ExtractVVoucher(empty) = %q, want \"\"", got)
	}
	if got := ExtractVVoucher([]byte(`{}`)); got != "" {
		t.Fatalf("ExtractVVoucher(no data) = %q, want \"\"", got)
	}
	if got := ExtractVVoucher([]byte(`{"data":{}}`)); got != "" {
		t.Fatalf("ExtractVVoucher(no v_voucher) = %q, want \"\"", got)
	}
}

type captchaRoundTripper func(*http.Request) (*http.Response, error)

func (fn captchaRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func captchaJSONResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newTestCaptchaClient(handler func(*http.Request) (*http.Response, error)) *CaptchaClient {
	identity := NewIdentityProvider(func() time.Time {
		return time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	})
	return NewCaptchaClient(captchaRoundTripper(handler), identity)
}

func TestRegisterChallengeSuccess(t *testing.T) {
	t.Parallel()
	client := newTestCaptchaClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/x/gaia-vgate/v1/register" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		return captchaJSONResponse(`{"code":0,"data":{"gt":"abc123gt","challenge":"def456challenge","key":"key789","type":"fullpage"}}`), nil
	})
	challenge, err := client.RegisterChallenge(context.Background(), "test_voucher", "test_cookie")
	if err != nil {
		t.Fatalf("RegisterChallenge: %v", err)
	}
	if challenge.GT != "abc123gt" {
		t.Fatalf("GT = %q, want abc123gt", challenge.GT)
	}
	if challenge.Challenge != "def456challenge" {
		t.Fatalf("Challenge = %q, want def456challenge", challenge.Challenge)
	}
	if challenge.Key != "key789" {
		t.Fatalf("Key = %q, want key789", challenge.Key)
	}
}

func TestRegisterChallengeErrorCode(t *testing.T) {
	t.Parallel()
	client := newTestCaptchaClient(func(req *http.Request) (*http.Response, error) {
		return captchaJSONResponse(`{"code":-400,"message":"invalid v_voucher","data":{}}`), nil
	})
	_, err := client.RegisterChallenge(context.Background(), "bad_voucher", "test_cookie")
	if err == nil {
		t.Fatal("expected error for non-zero code")
	}
	biliErr := asBilibiliError(err)
	if biliErr == nil || biliErr.Kind != ErrorCaptcha {
		t.Fatalf("expected ErrorCaptcha, got %v", err)
	}
}

func TestValidateSuccess(t *testing.T) {
	t.Parallel()
	client := newTestCaptchaClient(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/x/gaia-vgate/v1/validate" {
			t.Fatalf("unexpected path: %s", req.URL.Path)
		}
		body, _ := io.ReadAll(req.Body)
		_ = body
		return captchaJSONResponse(`{"code":0,"data":{"grisk_id":"grisk_test_123"}}`), nil
	})
	challenge := &CaptchaChallenge{
		GT:        "gt_test",
		Challenge: "challenge_test",
		Key:       "key_test",
		Type:      "fullpage",
	}
	result, err := client.Validate(context.Background(), challenge, "token_test", "validate_test", "seccode_test", "csrf_test", "test_cookie")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if result.GriskID != "grisk_test_123" {
		t.Fatalf("GriskID = %q, want grisk_test_123", result.GriskID)
	}
}

func TestValidateErrorCode(t *testing.T) {
	t.Parallel()
	client := newTestCaptchaClient(func(req *http.Request) (*http.Response, error) {
		return captchaJSONResponse(`{"code":-400,"message":"invalid token","data":{}}`), nil
	})
	challenge := &CaptchaChallenge{
		GT:        "gt_test",
		Challenge: "challenge_test",
		Key:       "key_test",
		Type:      "fullpage",
	}
	_, err := client.Validate(context.Background(), challenge, "bad_token", "bad_validate", "bad_seccode", "csrf_test", "test_cookie")
	if err == nil {
		t.Fatal("expected error for non-zero code")
	}
}

func TestNewCaptchaClient(t *testing.T) {
	t.Parallel()
	identity := NewIdentityProvider(nil)
	client := NewCaptchaClient(nil, identity)
	if client == nil {
		t.Fatal("NewCaptchaClient returned nil")
	}
	if client.client == nil {
		t.Fatal("CaptchaClient has nil http client")
	}
	if client.identity != identity {
		t.Fatal("CaptchaClient identity mismatch")
	}
}

func TestValidatePostsCorrectFormData(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	client := newTestCaptchaClient(func(req *http.Request) (*http.Response, error) {
		capturedBody, _ = io.ReadAll(req.Body)
		return captchaJSONResponse(`{"code":0,"data":{"grisk_id":"grisk_ok"}}`), nil
	})
	challenge := &CaptchaChallenge{
		GT:        "my_gt",
		Challenge: "my_challenge",
		Key:       "my_key",
		Type:      "my_type",
	}
	_, err := client.Validate(context.Background(), challenge, "my_token", "my_validate", "my_seccode|jordan", "my_csrf", "my_cookie")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	formStr := string(capturedBody)
	for _, want := range []string{"gt=my_gt", "challenge=my_challenge", "key=my_key", "type=my_type", "token=my_token", "validate=my_validate", "csrf=my_csrf"} {
		if !strings.Contains(formStr, want) {
			t.Fatalf("form body missing %q in %q", want, formStr)
		}
	}
}

func TestTrySolveUsesCookieForCSRF(t *testing.T) {
	t.Parallel()
	solveCalled := false
	identity := NewIdentityProvider(func() time.Time {
		return time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	})
	// Use a custom captcha client that intercepts at the Validate level.
	client := &CaptchaClient{
		client: &http.Client{
			Transport: captchaRoundTripper(func(req *http.Request) (*http.Response, error) {
				// geetest JS fetch
				if strings.Contains(req.URL.Host, "static.geetest.com") {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"application/javascript"}},
						Body:       io.NopCloser(strings.NewReader(`var o={"static_key":"deadbeefcafebabe"};`)),
					}, nil
				}
				// register
				if req.URL.Path == "/x/gaia-vgate/v1/register" {
					return captchaJSONResponse(`{"code":0,"data":{"gt":"test_gt","challenge":"test_challenge","key":"test_key","type":"fullpage"}}`), nil
				}
				// validate
				if req.URL.Path == "/x/gaia-vgate/v1/validate" {
					solveCalled = true
					body, _ := io.ReadAll(req.Body)
					formStr := string(body)
					if !strings.Contains(formStr, "csrf=test_csrf_from_cookie") {
						t.Fatalf("validate missing csrf in form: %s", formStr)
					}
					return captchaJSONResponse(`{"code":0,"data":{"grisk_id":"solved_grisk"}}`), nil
				}
				return captchaJSONResponse(`{}`), nil
			}),
			Timeout: defaultRequestTimeout,
		},
		identity: identity,
	}
	result, err := client.TrySolve(context.Background(), "test_voucher", "bili_jct=test_csrf_from_cookie; SESSDATA=test;")
	if err != nil {
		t.Fatalf("TrySolve: %v", err)
	}
	if !solveCalled {
		t.Fatal("Validate was not called")
	}
	if result.GriskID != "solved_grisk" {
		t.Fatalf("GriskID = %q, want solved_grisk", result.GriskID)
	}
}

func TestComputeGeetestResponseFormat(t *testing.T) {
	t.Parallel()
	challenge := &CaptchaChallenge{
		GT:        "abc123gt",
		Challenge: "def456challenge",
	}
	token, validate, seccode, err := computeGeetestResponse(challenge, "testkey12345678")
	if err != nil {
		t.Fatalf("computeGeetestResponse: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}
	if validate == "" {
		t.Fatal("validate is empty")
	}
	if len(validate) != 16 {
		t.Fatalf("validate length = %d, want 16", len(validate))
	}
	if !strings.HasSuffix(seccode, "|jordan") {
		t.Fatalf("seccode should end with |jordan: %q", seccode)
	}
	if !strings.HasPrefix(seccode, validate) {
		t.Fatalf("seccode should start with validate: %q vs %q", seccode, validate)
	}
}
