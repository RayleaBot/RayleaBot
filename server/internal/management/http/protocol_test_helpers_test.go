package managementhttp

import (
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/adapter"
	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

func defaultAdapterTestConfig() config.AdapterConfig {
	return config.AdapterConfig{
		ConnectTimeoutSeconds:   15,
		ReconnectInitialSeconds: 2,
		ReconnectMultiplier:     2,
		ReconnectMaxSeconds:     120,
		ReconnectJitterRatio:    0.2,
	}
}

func waitForAdapterState(t *testing.T, shell *adapter.Shell, want adapter.State, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if shell.Snapshot().State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for adapter state %s, got %s", want, shell.Snapshot().State)
}

func waitForRuntimeInfo(t *testing.T, shell *adapter.Shell, transport adapter.TransportKey, wantProvider string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snapshot := shell.Snapshot()
		var info adapter.TransportRuntimeInfo
		switch transport {
		case adapter.TransportForwardWS:
			info = snapshot.ForwardWS.RuntimeInfo
		case adapter.TransportReverseWS:
			info = snapshot.ReverseWS.RuntimeInfo
		case adapter.TransportHTTPAPI:
			info = snapshot.HTTPAPI.RuntimeInfo
		case adapter.TransportWebhook:
			info = snapshot.Webhook.RuntimeInfo
		}
		if info.Provider == wantProvider {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s runtime provider %s, got %#v", transport, wantProvider, shell.Snapshot())
}
