package market

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDefaultCatalogClientBlocksPrivateConnections(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client := newCatalogHTTPClient()
	defer client.CloseIdleConnections()
	response, err := client.Get(server.URL)
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil || requests.Load() != 0 {
		t.Fatalf("private connection error=%v requests=%d", err, requests.Load())
	}
}

func TestCatalogDialRejectsNonPublicDNSAnswersBeforeConnecting(t *testing.T) {
	for _, address := range []string{"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1", "169.254.169.254", "100.64.0.1", "0.1.2.3", "240.0.0.1", "::", "::1", "fc00::1", "fe80::1", "ff02::1", "::ffff:127.0.0.1"} {
		t.Run(address, func(t *testing.T) {
			dial := publicCatalogDialer(func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr(address)}, nil
			}, func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("connected before validating all DNS answers")
				return nil, nil
			})
			if _, err := dial(t.Context(), "tcp", "catalog.example:443"); err == nil {
				t.Fatal("accepted private DNS answer")
			}
		})
	}
}

func TestCatalogDialPinsResolvedIPAndPropagatesErrors(t *testing.T) {
	lookupCount := 0
	sentinel := errors.New("connection refused")
	dial := publicCatalogDialer(func(_ context.Context, network, host string) ([]netip.Addr, error) {
		lookupCount++
		if network != "ip" || host != "catalog.example" {
			t.Fatalf("lookup %s %s", network, host)
		}
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}, func(_ context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != "8.8.8.8:443" {
			t.Fatalf("unvalidated dial %s %s", network, address)
		}
		return nil, sentinel
	})
	if _, err := dial(t.Context(), "tcp", "catalog.example:443"); !errors.Is(err, sentinel) || lookupCount != 1 {
		t.Fatalf("dial error=%v lookups=%d", err, lookupCount)
	}
	for _, lookupErr := range []error{nil, context.Canceled} {
		dial := publicCatalogDialer(func(context.Context, string, string) ([]netip.Addr, error) { return nil, lookupErr },
			func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("dialed without DNS answer")
				return nil, nil
			})
		if _, err := dial(t.Context(), "tcp", "catalog.example:443"); err == nil || lookupErr != nil && !errors.Is(err, lookupErr) {
			t.Fatalf("lookup error lost: %v", err)
		}
	}
}

func TestCatalogFetchChecksRedirectBeforeSending(t *testing.T) {
	for _, target := range []string{"https://127.0.0.1/catalog", "https://[::ffff:127.0.0.1]/catalog", "http://public.example/catalog", "https://user:pass@public.example/catalog"} {
		t.Run(target, func(t *testing.T) {
			calls := 0
			service := newTestService(t, emptyCatalog{}, nil, newMemoryRepository(nil), catalogRedirectTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{target}}, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
			}))
			if _, err := service.fetch(t.Context(), "https://public.example/catalog"); err == nil || calls != 1 {
				t.Fatalf("redirect error=%v requests=%d", err, calls)
			}
		})
	}
}

type catalogRedirectTransport func(*http.Request) (*http.Response, error)

func (transport catalogRedirectTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}
