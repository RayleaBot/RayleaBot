package session

import (
	"net/http"

	"testing"
)

func TestValidateCookieForLoginRequiresSESSDATA(t *testing.T) {
	t.Parallel()

	err := validateCookieForLogin("bili_jct=csrf;")
	if err == nil {
		t.Fatalf("expected missing SESSDATA error")
	}
	biliErr := asBilibiliError(err)
	if biliErr == nil || biliErr.Kind != ErrorAuth {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestClassifyBilibiliRiskAndAuthErrors(t *testing.T) {
	t.Parallel()

	risk := apiError(http.StatusOK, -352, "风控校验失败", []byte(`{"code":-352}`))
	if biliErr := asBilibiliError(risk); biliErr == nil || biliErr.Kind != ErrorRiskControl {
		t.Fatalf("unexpected risk error: %#v", risk)
	}
	auth := apiError(http.StatusOK, -101, "账号未登录", []byte(`{"code":-101}`))
	if biliErr := asBilibiliError(auth); biliErr == nil || biliErr.Kind != ErrorAuth {
		t.Fatalf("unexpected auth error: %#v", auth)
	}
	csrf := apiError(http.StatusOK, -111, "csrf 校验失败", []byte(`{"code":-111}`))
	if biliErr := asBilibiliError(csrf); biliErr == nil || biliErr.Kind != ErrorCSRF {
		t.Fatalf("unexpected csrf error: %#v", csrf)
	}
	rateLimit := apiError(http.StatusOK, -509, "请求过于频繁", []byte(`{"code":-509}`))
	if biliErr := asBilibiliError(rateLimit); biliErr == nil || biliErr.Kind != ErrorRateLimit {
		t.Fatalf("unexpected rate limit error: %#v", rateLimit)
	}
	serverErr := apiError(http.StatusInternalServerError, 0, "", []byte(`{"code":0}`))
	if biliErr := asBilibiliError(serverErr); biliErr == nil || biliErr.Kind != ErrorServer {
		t.Fatalf("unexpected server error: %#v", serverErr)
	}
}
