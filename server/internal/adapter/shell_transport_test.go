package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func TestCallAPIAnyHTTPFallbackClearsAuthIssueAfterSuccess(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"retcode": 0,
			"data": map[string]any{
				"user_id": 123456,
			},
			"echo": "http-success",
		}); err != nil {
			t.Errorf("encode HTTP API response: %v", err)
		}
	}))
	defer server.Close()

	shell := newTestShell(config.OneBotConfig{
		HTTPAPI: config.OneBotTransportConfig{
			Enabled:     true,
			URL:         server.URL,
			AccessToken: "test-token",
		},
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	_, err := shell.CallAPIAny(context.Background(), "get_login_info", nil)
	if err == nil {
		t.Fatal("expected first HTTP API call to fail")
	}
	var adapterErr *Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected *adapter.Error, got %T", err)
	}
	if adapterErr.Code != errorCodeHTTPAPIAuthFailed {
		t.Fatalf("unexpected adapter error code: got %q want %q", adapterErr.Code, errorCodeHTTPAPIAuthFailed)
	}

	failedSnapshot := shell.Snapshot()
	if failedSnapshot.HTTPAPI.LastErrorCode != errorCodeHTTPAPIAuthFailed {
		t.Fatalf("unexpected HTTP API error code after failure: got %q want %q", failedSnapshot.HTTPAPI.LastErrorCode, errorCodeHTTPAPIAuthFailed)
	}
	if failedSnapshot.LastErrorCode != errorCodeHTTPAPIAuthFailed {
		t.Fatalf("unexpected aggregate error code after failure: got %q want %q", failedSnapshot.LastErrorCode, errorCodeHTTPAPIAuthFailed)
	}

	result, err := shell.CallAPIAny(context.Background(), "get_login_info", nil)
	if err != nil {
		t.Fatalf("expected second HTTP API call to succeed: %v", err)
	}
	resultMap, ok := result.(map[string]any)
	if !ok || resultMap["user_id"] != "123456" {
		t.Fatalf("unexpected HTTP API result: %#v", result)
	}

	recoveredSnapshot := shell.Snapshot()
	if recoveredSnapshot.HTTPAPI.State != TransportStateConnected {
		t.Fatalf("unexpected HTTP API state after recovery: got %s want %s", recoveredSnapshot.HTTPAPI.State, TransportStateConnected)
	}
	if recoveredSnapshot.HTTPAPI.LastErrorCode != "" || recoveredSnapshot.HTTPAPI.LastErrorMessage != "" {
		t.Fatalf("expected cleared HTTP API transport error, got %#v", recoveredSnapshot.HTTPAPI)
	}
	if recoveredSnapshot.LastErrorCode != "" || recoveredSnapshot.LastErrorMessage != "" {
		t.Fatalf("expected cleared aggregate error after HTTP API recovery, got %#v", recoveredSnapshot)
	}
}

func TestAcceptWebhookPayloadClearsInvalidPayloadIssueAfterSuccess(t *testing.T) {
	t.Parallel()

	shell := newTestShell(config.OneBotConfig{
		Webhook: config.OneBotTransportConfig{
			Enabled: true,
			URL:     "http://127.0.0.1:8080/onebot",
		},
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})

	err := shell.AcceptWebhookPayload(context.Background(), []byte("{"))
	if err == nil {
		t.Fatal("expected invalid webhook payload to fail")
	}
	var adapterErr *Error
	if !errors.As(err, &adapterErr) {
		t.Fatalf("expected *adapter.Error, got %T", err)
	}
	if adapterErr.Code != errorCodeWebhookInvalidPayload {
		t.Fatalf("unexpected webhook error code: got %q want %q", adapterErr.Code, errorCodeWebhookInvalidPayload)
	}

	failedSnapshot := shell.Snapshot()
	if failedSnapshot.Webhook.LastErrorCode != errorCodeWebhookInvalidPayload {
		t.Fatalf("unexpected webhook error code after failure: got %q want %q", failedSnapshot.Webhook.LastErrorCode, errorCodeWebhookInvalidPayload)
	}

	err = shell.AcceptWebhookPayload(context.Background(), []byte(`{"post_type":"meta_event","meta_event_type":"lifecycle","sub_type":"enable"}`))
	if err != nil {
		t.Fatalf("expected webhook payload recovery to succeed: %v", err)
	}

	recoveredSnapshot := shell.Snapshot()
	if recoveredSnapshot.Webhook.State != TransportStateListening {
		t.Fatalf("unexpected webhook state after recovery: got %s want %s", recoveredSnapshot.Webhook.State, TransportStateListening)
	}
	if recoveredSnapshot.Webhook.LastErrorCode != "" || recoveredSnapshot.Webhook.LastErrorMessage != "" {
		t.Fatalf("expected cleared webhook transport error, got %#v", recoveredSnapshot.Webhook)
	}
	if recoveredSnapshot.LastErrorCode != "" || recoveredSnapshot.LastErrorMessage != "" {
		t.Fatalf("expected cleared aggregate error after webhook recovery, got %#v", recoveredSnapshot)
	}
}

func TestSyncLastErrorLockedClearsRecoveredReverseWSIssue(t *testing.T) {
	t.Parallel()

	shell := newTestShell(config.OneBotConfig{
		ReverseWS: config.OneBotTransportConfig{
			Enabled: true,
			URL:     "ws://127.0.0.1:8080/onebot/reverse",
		},
	}, shellDeps{
		connectTimeout: 75 * time.Millisecond,
		sleep:          blockingSleep,
	})
	shell.MarkReverseWSAuthFailed()

	shell.mu.Lock()
	shell.snapshot.ReverseWS.State = TransportStateConnected
	shell.snapshot.ReverseWS.LastErrorCode = ""
	shell.snapshot.ReverseWS.LastErrorMessage = ""
	shell.syncLastErrorLocked()
	snapshot := cloneSnapshot(shell.snapshot)
	shell.mu.Unlock()

	if snapshot.ReverseWS.LastErrorCode != "" || snapshot.ReverseWS.LastErrorMessage != "" {
		t.Fatalf("expected cleared reverse websocket transport error, got %#v", snapshot.ReverseWS)
	}
	if snapshot.LastErrorCode != "" || snapshot.LastErrorMessage != "" {
		t.Fatalf("expected cleared aggregate error after reverse websocket recovery, got %#v", snapshot)
	}
}
