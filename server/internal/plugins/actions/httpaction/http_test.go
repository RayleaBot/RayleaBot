package httpaction

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	runtimeaction "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/action"
	runtimemanager "github.com/RayleaBot/RayleaBot/server/internal/plugins/runtime/manager"
)

func TestExecuteAppliesCredentialInjectorAndMarksAfterSuccess(t *testing.T) {
	t.Parallel()

	var marked bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Credential"); got != "fixture" {
			t.Fatalf("unexpected credential header: %q", got)
		}
		if got := r.URL.Query().Get("signed"); got != "1" {
			t.Fatalf("unexpected signed query: %q", got)
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))
	defer server.Close()

	result, err := Execute(context.Background(), Request{
		PluginID: "plugin.http",
		Action: runtimeaction.Action{
			HTTPMethod: "GET",
			HTTPURL:    server.URL + "/v1/data",
		},
		Config: config.Config{
			HTTP: config.HTTPConfig{
				TimeoutSeconds:    5,
				MaxRetries:        0,
				AllowPrivateHosts: []string{"127.0.0.1"},
			},
		},
		Capabilities: stubHTTPCapabilities{
			capabilities: map[string]bool{"http.request": true},
			httpHosts:    []string{"127.0.0.1"},
		},
		CredentialInjector: stubCredentialInjector{
			inject: func(_ context.Context, req CredentialRequest) (CredentialResult, error) {
				req.Headers["X-Credential"] = "fixture"
				return CredentialResult{
					URL: req.RawURL + "?signed=1",
					AfterSuccess: func(context.Context) error {
						marked = true
						return nil
					},
				}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !marked {
		t.Fatalf("expected credential success callback")
	}
	if result["status_code"] != http.StatusOK || result["body_text"] != "ok" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExecuteSkipsCredentialSuccessCallbackWhenRequestFails(t *testing.T) {
	t.Parallel()

	var marked bool
	_, err := Execute(context.Background(), Request{
		PluginID: "plugin.http",
		Action: runtimeaction.Action{
			HTTPMethod: "GET",
			HTTPURL:    "https://api.example.test/v1/data",
		},
		Config: config.Config{HTTP: config.HTTPConfig{TimeoutSeconds: 5, MaxRetries: 0}},
		Capabilities: stubHTTPCapabilities{
			capabilities: map[string]bool{"http.request": true},
			httpHosts:    []string{"other.example.test"},
		},
		CredentialInjector: stubCredentialInjector{
			inject: func(context.Context, CredentialRequest) (CredentialResult, error) {
				return CredentialResult{
					AfterSuccess: func(context.Context) error {
						marked = true
						return nil
					},
				}, nil
			},
		},
	})

	var runtimeErr *runtimemanager.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %#v", err)
	}
	if runtimeErr.Code != "plugin.capability_violation" {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
	if marked {
		t.Fatalf("credential success callback ran after failed request")
	}
}

func TestExecuteReturnsCredentialInjectorError(t *testing.T) {
	t.Parallel()

	_, err := Execute(context.Background(), Request{
		PluginID: "plugin.http",
		Action: runtimeaction.Action{
			HTTPMethod: "GET",
			HTTPURL:    "https://api.example.test/v1/data",
		},
		Config: config.Config{HTTP: config.HTTPConfig{TimeoutSeconds: 5, MaxRetries: 0}},
		Capabilities: stubHTTPCapabilities{
			capabilities: map[string]bool{"http.request": true},
			httpHosts:    []string{"api.example.test"},
		},
		CredentialInjector: stubCredentialInjector{
			inject: func(context.Context, CredentialRequest) (CredentialResult, error) {
				return CredentialResult{}, errors.New("sign failed")
			},
		},
	})

	var runtimeErr *runtimemanager.Error
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected runtime error, got %#v", err)
	}
	if runtimeErr.Code != "plugin.internal_error" || !strings.Contains(runtimeErr.Error(), "sign failed") {
		t.Fatalf("unexpected runtime error: %#v", runtimeErr)
	}
}

type stubHTTPCapabilities struct {
	capabilities map[string]bool
	httpHosts    []string
}

func (s stubHTTPCapabilities) CapabilityDeclared(_ context.Context, _ string, capability string) bool {
	return s.capabilities[capability]
}

func (s stubHTTPCapabilities) HTTPHosts(context.Context, string) []string {
	return append([]string(nil), s.httpHosts...)
}

type stubCredentialInjector struct {
	inject func(context.Context, CredentialRequest) (CredentialResult, error)
}

func (s stubCredentialInjector) Inject(ctx context.Context, req CredentialRequest) (CredentialResult, error) {
	if s.inject == nil {
		return CredentialResult{}, nil
	}
	return s.inject(ctx, req)
}
