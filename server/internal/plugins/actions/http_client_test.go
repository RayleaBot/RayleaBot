package actions

import (
	"compress/gzip"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPClientAllowsPrivateHostAndDoesNotFollowRedirect(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", "/final")
			w.WriteHeader(http.StatusFound)
			return
		}
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Write response body failed: %v", err)
		}
	}))
	defer server.Close()

	requestURL, resolver := testHTTPURLAndResolver(t, server.URL, "internal.test")
	client := newHTTPClient(httpClientConfig{
		Resolver:          resolver,
		Timeout:           5 * time.Second,
		MaxRetries:        0,
		AllowPrivateHosts: []string{"internal.test"},
	})

	response, err := client.do(context.Background(), httpClientRequest{
		Method: "GET",
		URL:    requestURL + "/redirect",
	}, []string{"internal.test"})
	if err != nil {
		t.Fatalf("Do redirect request: %v", err)
	}
	if response.StatusCode != http.StatusFound {
		t.Fatalf("StatusCode = %d, want %d", response.StatusCode, http.StatusFound)
	}
}

func TestHTTPClientRejectsPrivateHostWithoutAllowlist(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Write response body failed: %v", err)
		}
	}))
	defer server.Close()

	requestURL, resolver := testHTTPURLAndResolver(t, server.URL, "internal.test")
	client := newHTTPClient(httpClientConfig{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	})

	_, err := client.do(context.Background(), httpClientRequest{
		Method: "GET",
		URL:    requestURL,
	}, []string{"internal.test"})
	if !errors.Is(err, errHTTPScopeViolation) {
		t.Fatalf("Do private request error = %v, want errHTTPScopeViolation", err)
	}
}

func TestAuthorizeResolvedAddrsAllowsFakeIPDNSRanges(t *testing.T) {
	t.Parallel()

	addresses := []netip.Addr{
		netip.MustParseAddr("198.18.0.112"),
		netip.MustParseAddr("fdfe:dcba:9876::3d"),
	}
	if err := authorizeResolvedAddrs(addresses, false, true); err != nil {
		t.Fatalf("authorizeResolvedAddrs returned error for fake-ip DNS ranges: %v", err)
	}
}

func TestAuthorizeResolvedAddrsRejectsPrivateDNSResult(t *testing.T) {
	t.Parallel()

	for _, address := range []string{"10.0.0.1", "fd00::1"} {
		if err := authorizeResolvedAddrs([]netip.Addr{netip.MustParseAddr(address)}, false, true); !errors.Is(err, errHTTPScopeViolation) {
			t.Fatalf("authorizeResolvedAddrs private DNS result %s error = %v, want errHTTPScopeViolation", address, err)
		}
	}
}

func TestHTTPClientRejectsLiteralFakeIPWithoutAllowlist(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(httpClientConfig{
		Timeout: 5 * time.Second,
	})

	for _, item := range []struct {
		url  string
		host string
	}{
		{url: "https://198.18.0.112/resource", host: "198.18.0.112"},
		{url: "https://[fdfe:dcba:9876::3d]/resource", host: "fdfe:dcba:9876::3d"},
	} {
		_, err := client.do(context.Background(), httpClientRequest{
			Method: "GET",
			URL:    item.url,
		}, []string{item.host})
		if !errors.Is(err, errHTTPScopeViolation) {
			t.Fatalf("Do literal fake-ip request %s error = %v, want errHTTPScopeViolation", item.url, err)
		}
	}
}

func TestHTTPClientRetriesIdempotentStatusCodes(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("recovered")); err != nil {
			t.Errorf("Write response body failed: %v", err)
		}
	}))
	defer server.Close()

	requestURL, resolver := testHTTPURLAndResolver(t, server.URL, "internal.test")
	client := newHTTPClient(httpClientConfig{
		Resolver:          resolver,
		Timeout:           5 * time.Second,
		MaxRetries:        1,
		AllowPrivateHosts: []string{"internal.test"},
	})

	response, err := client.do(context.Background(), httpClientRequest{
		Method: "GET",
		URL:    requestURL,
	}, []string{"internal.test"})
	if err != nil {
		t.Fatalf("Do retry request: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("hits = %d, want 2", got)
	}
}

func TestHTTPClientRejectsDecodedBodyLimitWithoutRetry(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusServiceUnavailable)
		writer := gzip.NewWriter(w)
		_, _ = writer.Write([]byte(strings.Repeat("a", 128)))
		_ = writer.Close()
	}))
	defer server.Close()

	requestURL, resolver := testHTTPURLAndResolver(t, server.URL, "internal.test")
	client := newHTTPClient(httpClientConfig{
		Resolver:             resolver,
		Timeout:              5 * time.Second,
		MaxRetries:           2,
		MaxResponseBodyBytes: 64,
		AllowPrivateHosts:    []string{"internal.test"},
	})

	_, err := client.do(context.Background(), httpClientRequest{
		Method:  "GET",
		URL:     requestURL,
		Headers: map[string]string{"Accept-Encoding": "gzip"},
	}, []string{"internal.test"})
	if !errors.Is(err, errHTTPResponseTooLarge) {
		t.Fatalf("decoded body limit error = %v, want errHTTPResponseTooLarge", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("oversized response was retried: hits=%d want=1", got)
	}
}

func TestHTTPClientRejectsHeaderLimitWithoutRetry(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("X-Large", strings.Repeat("h", int(maxHTTPResponseHeaderBytes)))
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	requestURL, resolver := testHTTPURLAndResolver(t, server.URL, "internal.test")
	client := newHTTPClient(httpClientConfig{
		Resolver:          resolver,
		Timeout:           5 * time.Second,
		MaxRetries:        2,
		AllowPrivateHosts: []string{"internal.test"},
	})

	_, err := client.do(context.Background(), httpClientRequest{Method: "GET", URL: requestURL}, []string{"internal.test"})
	if !errors.Is(err, errHTTPResponseTooLarge) {
		t.Fatalf("header limit error = %v, want errHTTPResponseTooLarge", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("oversized headers were retried: hits=%d want=1", got)
	}
}

func TestHTTPClientRejectsPlainHTTPForPublicHost(t *testing.T) {
	t.Parallel()

	client := newHTTPClient(httpClientConfig{
		Resolver: staticHTTPResolver{
			"api.example.test": {{
				IP: net.ParseIP("93.184.216.34"),
			}},
		},
		Timeout: 5 * time.Second,
	})

	_, err := client.do(context.Background(), httpClientRequest{
		Method: "GET",
		URL:    "http://api.example.test/resource",
	}, []string{"api.example.test"})
	if !errors.Is(err, errHTTPInvalidRequest) {
		t.Fatalf("Do public http request error = %v, want errHTTPInvalidRequest", err)
	}
}

type staticHTTPResolver map[string][]net.IPAddr

func (r staticHTTPResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	items, ok := r[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host}
	}
	return items, nil
}

func testHTTPURLAndResolver(t *testing.T, rawURL string, host string) (string, httpResolver) {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Parse URL %q: %v", rawURL, err)
	}
	listenerHost, _, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		t.Fatalf("SplitHostPort %q: %v", parsedURL.Host, err)
	}
	return parsedURL.Scheme + "://" + host + ":" + parsedURL.Port(), staticHTTPResolver{
		host: []net.IPAddr{{
			IP: net.ParseIP(listenerHost),
		}},
	}
}
