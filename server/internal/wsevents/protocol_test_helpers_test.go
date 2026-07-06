package wsevents

import (
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
	"github.com/RayleaBot/RayleaBot/server/internal/onebot11"
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

func waitForAdapterState(t *testing.T, shell *onebot11.Shell, want onebot11.State, timeout time.Duration) {
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

func waitForRuntimeInfo(t *testing.T, shell *onebot11.Shell, transport onebot11.TransportKey, wantProvider string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		snapshot := shell.Snapshot()
		var info onebot11.TransportRuntimeInfo
		switch transport {
		case onebot11.TransportForwardWS:
			info = snapshot.ForwardWS.RuntimeInfo
		case onebot11.TransportReverseWS:
			info = snapshot.ReverseWS.RuntimeInfo
		case onebot11.TransportHTTPAPI:
			info = snapshot.HTTPAPI.RuntimeInfo
		case onebot11.TransportWebhook:
			info = snapshot.Webhook.RuntimeInfo
		}
		if info.Provider == wantProvider {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s runtime provider %s, got %#v", transport, wantProvider, shell.Snapshot())
}
