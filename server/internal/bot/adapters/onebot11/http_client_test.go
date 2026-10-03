package onebot11

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/config"
)

const httpPoolTestResponse = `{"status":"ok","retcode":0,"data":{"value":"fixture"}}`

func newHTTPPoolTestShell(t *testing.T, endpoint string) *Shell {
	t.Helper()
	shell := NewForTest("fixture", config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: endpoint}}, config.AdapterConfig{ConnectTimeoutSeconds: 5}, nil, true)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shell.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	return shell
}

func awaitHTTPPoolEvent(t *testing.T, events <-chan struct{}) {
	t.Helper()
	select {
	case <-events:
	case <-time.After(5 * time.Second):
		t.Fatal("HTTP connection event did not arrive")
	}
}

func TestHTTPAPIReusesConcurrentConnectionsAndClosesIdleOnStop(t *testing.T) {
	t.Parallel()
	const concurrency = 8
	entered := make(chan struct{}, concurrency)
	closed := make(chan struct{}, 2*concurrency)
	gates := []chan struct{}{make(chan struct{}), make(chan struct{})}
	var connections atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request APICallRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		round, err := strconv.Atoi(request.Echo)
		if err != nil || round < 0 || round >= len(gates) {
			t.Errorf("unexpected request echo %q", request.Echo)
			return
		}
		entered <- struct{}{}
		select {
		case <-gates[round]:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, httpPoolTestResponse)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			connections.Add(1)
		}
		if state == http.StateClosed {
			closed <- struct{}{}
		}
	}
	server.Start()
	t.Cleanup(server.Close)
	shell := newHTTPPoolTestShell(t, server.URL)
	for round := range gates {
		release := sync.OnceFunc(func() { close(gates[round]) })
		t.Cleanup(release)
		results := make(chan error, concurrency)
		for range concurrency {
			go func() {
				_, err := shell.doHTTPAPIRequest(t.Context(), APICallRequest{Action: "get_status", Echo: strconv.Itoa(round)})
				results <- err
			}()
		}
		for range concurrency {
			awaitHTTPPoolEvent(t, entered)
		}
		release()
		for range concurrency {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		if got := connections.Load(); got != concurrency {
			t.Fatalf("round %d created %d connections, want reuse of %d", round, got, concurrency)
		}
	}
	if err := shell.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	for range concurrency {
		awaitHTTPPoolEvent(t, closed)
	}
}

func TestHTTPAPINaturalExitClosesIdleConnections(t *testing.T) {
	t.Parallel()
	closed := make(chan struct{}, 4)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, httpPoolTestResponse) }))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			closed <- struct{}{}
		}
	}
	server.Start()
	t.Cleanup(server.Close)
	shell := newHTTPPoolTestShell(t, server.URL)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	shell.Start(ctx)
	if _, err := shell.doHTTPAPIRequest(t.Context(), APICallRequest{Action: "get_status"}); err != nil {
		t.Fatal(err)
	}
	cancel()
	awaitHTTPPoolEvent(t, closed)
}

func TestHTTPAPIStopLeavesActiveRequestAlive(t *testing.T) {
	t.Parallel()
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = io.WriteString(w, httpPoolTestResponse)
	}))
	t.Cleanup(server.Close)
	shell := newHTTPPoolTestShell(t, server.URL)
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)
	result := make(chan error, 1)
	go func() {
		_, err := shell.doHTTPAPIRequest(t.Context(), APICallRequest{Action: "get_status"})
		result <- err
	}()
	awaitHTTPPoolEvent(t, entered)
	if err := shell.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	unblock()
	if err := <-result; err != nil {
		t.Fatalf("active request was interrupted by idle cleanup: %v", err)
	}
	if state := shell.Snapshot().HTTPAPI.State; state != TransportStateStopped {
		t.Fatalf("late request revived stopped transport: %s", state)
	}
}

func TestHTTPAPIReconfigureKeepsRequestGenerationAndStatus(t *testing.T) {
	t.Parallel()
	for _, oldStatus := range []int{http.StatusOK, http.StatusUnauthorized} {
		t.Run(strconv.Itoa(oldStatus), func(t *testing.T) {
			entered, release, closed := make(chan struct{}), make(chan struct{}), make(chan struct{}, 4)
			oldServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer fixture-old" {
					t.Errorf("old request authorization = %q", got)
				}
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
				w.WriteHeader(oldStatus)
				_, _ = io.WriteString(w, httpPoolTestResponse)
			}))
			oldServer.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateClosed {
					closed <- struct{}{}
				}
			}
			oldServer.Start()
			t.Cleanup(oldServer.Close)
			newStatus := http.StatusOK
			if oldStatus == http.StatusOK {
				newStatus = http.StatusUnauthorized
			}
			newServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer fixture-new" {
					t.Errorf("new request authorization = %q", got)
				}
				w.WriteHeader(newStatus)
				_, _ = io.WriteString(w, httpPoolTestResponse)
			}))
			t.Cleanup(newServer.Close)
			shell := NewForTest("fixture", config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: oldServer.URL, AccessToken: "fixture-old"}}, config.AdapterConfig{ConnectTimeoutSeconds: 5}, nil, true)
			t.Cleanup(func() {
				if err := shell.Stop(t.Context()); err != nil {
					t.Error(err)
				}
			})
			oldClient := shell.httpClient
			unblock := sync.OnceFunc(func() { close(release) })
			t.Cleanup(unblock)
			result := make(chan error, 1)
			go func() {
				_, err := shell.doHTTPAPIRequest(t.Context(), APICallRequest{Action: "get_status"})
				result <- err
			}()
			awaitHTTPPoolEvent(t, entered)
			if err := shell.Reload(config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: newServer.URL, AccessToken: "fixture-new"}}, config.AdapterConfig{ConnectTimeoutSeconds: 9}); err != nil {
				t.Fatal(err)
			}
			if shell.httpClient.Timeout != 9*time.Second || oldClient.Timeout != 5*time.Second {
				t.Fatal("reconfiguration mutated the active client's timeout")
			}
			_, currentErr := shell.doHTTPAPIRequest(t.Context(), APICallRequest{Action: "get_status"})
			if (currentErr != nil) != (newStatus != http.StatusOK) {
				t.Fatalf("new request = %v", currentErr)
			}
			current := shell.Snapshot().HTTPAPI
			unblock()
			if err := <-result; (err != nil) != (oldStatus != http.StatusOK) {
				t.Fatalf("old request result changed: %v", err)
			}
			if got := shell.Snapshot().HTTPAPI; got != current {
				t.Fatalf("old request changed new transport status: got=%+v want=%+v", got, current)
			}
			awaitHTTPPoolEvent(t, closed)
		})
	}
}

type blockingHTTPPoolParameter struct {
	entered chan<- struct{}
	release <-chan struct{}
}

func (parameter blockingHTTPPoolParameter) MarshalJSON() ([]byte, error) {
	parameter.entered <- struct{}{}
	<-parameter.release
	return []byte(`"fixture"`), nil
}

func TestHTTPAPIReconfigureClosesRetiredPoolOpenedAfterCapture(t *testing.T) {
	t.Parallel()
	closed := make(chan struct{}, 4)
	var oldRequests, newConnections atomic.Int32
	oldServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		oldRequests.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer fixture-old" {
			t.Errorf("captured request authorization = %q", got)
		}
		_, _ = io.WriteString(w, httpPoolTestResponse)
	}))
	oldServer.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			closed <- struct{}{}
		}
	}
	oldServer.Start()
	t.Cleanup(oldServer.Close)
	newServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer fixture-new" {
			t.Errorf("current request authorization = %q", got)
		}
		_, _ = io.WriteString(w, httpPoolTestResponse)
	}))
	newServer.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			newConnections.Add(1)
		}
	}
	newServer.Start()
	t.Cleanup(newServer.Close)
	shell := NewForTest("fixture", config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: oldServer.URL, AccessToken: "fixture-old"}}, config.AdapterConfig{ConnectTimeoutSeconds: 5}, nil, true)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shell.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	entered, release := make(chan struct{}, 1), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	t.Cleanup(unblock)
	result := make(chan error, 1)
	go func() {
		response, err := shell.doHTTPAPIRequest(t.Context(), APICallRequest{
			Action: "get_status", Params: map[string]any{"block": blockingHTTPPoolParameter{entered, release}},
		})
		if err == nil && response.Status != "ok" {
			err = errors.New("captured request lost its response")
		}
		result <- err
	}()
	awaitHTTPPoolEvent(t, entered)
	if err := shell.Reload(config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: newServer.URL, AccessToken: "fixture-new"}}, config.AdapterConfig{ConnectTimeoutSeconds: 5}); err != nil {
		t.Fatal(err)
	}
	if _, err := shell.doHTTPAPIRequest(t.Context(), APICallRequest{Action: "get_status"}); err != nil {
		t.Fatal(err)
	}
	unblock()
	if err := <-result; err != nil {
		t.Fatalf("late captured request failed: %v", err)
	}
	awaitHTTPPoolEvent(t, closed)
	if oldRequests.Load() != 1 {
		t.Fatalf("captured request did not use its original endpoint exactly once: %d", oldRequests.Load())
	}
	if _, err := shell.doHTTPAPIRequest(t.Context(), APICallRequest{Action: "get_status"}); err != nil {
		t.Fatal(err)
	}
	if newConnections.Load() != 1 {
		t.Fatal("retired request cleanup closed the current client's idle pool")
	}
}

func TestHTTPAPIDoesNotReplayAReceivedPOSTWithoutResponse(t *testing.T) {
	t.Parallel()
	var sendAttempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request APICallRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.Action == "send_msg" {
			sendAttempts.Add(1)
			connection, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_ = connection.Close()
			return
		}
		_, _ = io.WriteString(w, httpPoolTestResponse)
	}))
	t.Cleanup(server.Close)
	shell := newHTTPPoolTestShell(t, server.URL)
	if _, err := shell.doHTTPAPIRequest(t.Context(), APICallRequest{Action: "get_status"}); err != nil {
		t.Fatal(err)
	}
	_, err := shell.doHTTPAPIRequest(t.Context(), APICallRequest{Action: "send_msg"})
	var failure *Error
	if !errors.As(err, &failure) || failure.Code != ErrorCodeSendUnconfirmed || sendAttempts.Load() != 1 {
		t.Fatalf("unconfirmed POST was retried or misclassified: attempts=%d err=%v", sendAttempts.Load(), err)
	}
}

type customHTTPPoolTransport struct{ calls, closes atomic.Int32 }

func (transport *customHTTPPoolTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.calls.Add(1)
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(httpPoolTestResponse)), Request: request}, nil
}
func (transport *customHTTPPoolTransport) CloseIdleConnections() { transport.closes.Add(1) }

func TestHTTPAPIClientKeepsCustomTransportOwnership(t *testing.T) {
	t.Parallel()
	base := &customHTTPPoolTransport{}
	client, owned := newHTTPAPIClient(time.Second, base)
	cfg := config.OneBotConfig{HTTPAPI: config.OneBotTransportConfig{Enabled: true, URL: "http://fixture.invalid"}}
	shell := &Shell{cfg: cfg, httpClient: client, httpTransport: owned, snapshot: newTransportSnapshot(cfg)}
	if _, err := shell.doHTTPAPIRequest(t.Context(), APICallRequest{Action: "get_status"}); err != nil {
		t.Fatal(err)
	}
	if err := shell.Stop(t.Context()); err != nil {
		t.Fatal(err)
	}
	if base.calls.Load() != 1 || base.closes.Load() != 0 {
		t.Fatalf("custom transport was replaced or closed: calls=%d closes=%d", base.calls.Load(), base.closes.Load())
	}
}

func TestHTTPAPIClientPreservesTLSAndProxyConfiguration(t *testing.T) {
	t.Parallel()
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "trusted") }))
	t.Cleanup(tlsServer.Close)
	client, owned := newHTTPAPIClient(3*time.Second, tlsServer.Client().Transport)
	t.Cleanup(owned.CloseIdleConnections)
	response, err := client.Get(tlsServer.URL)
	if err != nil {
		t.Fatalf("cloned transport lost TLS configuration: %v", err)
	}
	_ = response.Body.Close()
	var proxyCalls atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Host != "fixture.invalid" {
			t.Errorf("unexpected proxy destination: %s", r.URL.Host)
		}
		proxyCalls.Add(1)
		_, _ = io.WriteString(w, "proxied")
	}))
	t.Cleanup(proxy.Close)
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = http.ProxyURL(proxyURL)
	proxyClient, proxyOwned := newHTTPAPIClient(3*time.Second, base)
	t.Cleanup(proxyOwned.CloseIdleConnections)
	response, err = proxyClient.Get("http://fixture.invalid/resource")
	if err != nil {
		t.Fatalf("cloned transport lost proxy configuration: %v", err)
	}
	_ = response.Body.Close()
	if proxyCalls.Load() != 1 {
		t.Fatal("request bypassed the configured proxy")
	}
}
