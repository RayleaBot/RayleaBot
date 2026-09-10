package actions

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

type changingHTTPResolver func(context.Context, string) ([]net.IPAddr, error)

func (f changingHTTPResolver) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return f(ctx, host)
}

func TestHTTPClientReauthorizesChangedDNSBeforeDial(t *testing.T) {
	t.Parallel()
	var lookups, requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer server.Close()
	address, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := newHTTPClient(httpClientConfig{
		Timeout: time.Second,
		Resolver: changingHTTPResolver(func(context.Context, string) ([]net.IPAddr, error) {
			ip := "93.184.216.34"
			if lookups.Add(1) >= 3 {
				ip = "127.0.0.1"
			}
			return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil
		}),
	})
	_, err = client.do(t.Context(), httpClientRequest{Method: "GET", URL: "https://rebinding.example:" + address.Port()})
	if !errors.Is(err, errHTTPInvalidRequest) || lookups.Load() != 3 || requests.Load() != 0 {
		t.Fatalf("DNS change reached protected endpoint: lookups=%d, requests=%d, err=%v", lookups.Load(), requests.Load(), err)
	}
}

func TestHTTPClientPrivateGrantDoesNotLeakToAnotherClient(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()
	address, resolver := testHTTPURLAndResolver(t, server.URL, "private.example")
	allowed := newHTTPClient(httpClientConfig{Resolver: resolver, AllowPrivateHosts: []string{"private.example"}})
	denied := newHTTPClient(httpClientConfig{Resolver: resolver})
	if _, err := allowed.do(t.Context(), httpClientRequest{Method: "GET", URL: address}); err != nil {
		t.Fatal(err)
	}
	if _, err := denied.do(t.Context(), httpClientRequest{Method: "GET", URL: address}); !errors.Is(err, errHTTPInvalidRequest) {
		t.Fatalf("private permission leaked between clients: %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("unauthorized request reached server: %d", requests.Load())
	}
}

// The pooled case measures a network-only lower bound. It intentionally has no
// domain/DNS authorization, body limits or retry policy and is not a replacement
// for the production client. Connection reuse requires preserving those checks.
func BenchmarkHTTPTransportIsolation(b *testing.B) {
	for _, pooled := range []bool{false, true} {
		name := "isolated_authorized"
		if pooled {
			name = "pooled_network_floor"
		}
		b.Run(name, func(b *testing.B) {
			var connections atomic.Int64
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "offline response")
			}))
			server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
				if state == http.StateNew {
					connections.Add(1)
				}
			}
			server.Start()
			defer server.Close()
			parsed, err := url.Parse(server.URL)
			if err != nil {
				b.Fatal(err)
			}
			client := newHTTPClient(httpClientConfig{AllowPrivateHosts: []string{parsed.Hostname()}, Timeout: time.Second})
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.Proxy = nil
			defer transport.CloseIdleConnections()
			baseline := &http.Client{Transport: transport, Timeout: time.Second}
			b.ResetTimer()
			for b.Loop() {
				if pooled {
					response, err := baseline.Get(server.URL)
					if err != nil {
						b.Fatal(err)
					}
					_, err = io.Copy(io.Discard, response.Body)
					closeErr := response.Body.Close()
					if err != nil || closeErr != nil {
						b.Fatal(errors.Join(err, closeErr))
					}
				} else if _, err := client.do(b.Context(), httpClientRequest{Method: "GET", URL: server.URL}); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(connections.Load())/float64(b.N), "connections/op")
		})
	}
}
