package pluginhttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientAllowsPrivateHostAndDoesNotFollowRedirect(t *testing.T) {
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

	requestURL, resolver := testURLAndResolver(t, server.URL, "internal.test")
	client := New(Config{
		Resolver:          resolver,
		Timeout:           5 * time.Second,
		MaxRetries:        0,
		AllowPrivateHosts: []string{"internal.test"},
	})

	response, err := client.Do(context.Background(), Request{
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

func TestClientRejectsPrivateHostWithoutAllowlist(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Errorf("Write response body failed: %v", err)
		}
	}))
	defer server.Close()

	requestURL, resolver := testURLAndResolver(t, server.URL, "internal.test")
	client := New(Config{
		Resolver: resolver,
		Timeout:  5 * time.Second,
	})

	_, err := client.Do(context.Background(), Request{
		Method: "GET",
		URL:    requestURL,
	}, []string{"internal.test"})
	if !errors.Is(err, ErrScopeViolation) {
		t.Fatalf("Do private request error = %v, want ErrScopeViolation", err)
	}
}

func TestClientRetriesIdempotentStatusCodes(t *testing.T) {
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

	requestURL, resolver := testURLAndResolver(t, server.URL, "internal.test")
	client := New(Config{
		Resolver:          resolver,
		Timeout:           5 * time.Second,
		MaxRetries:        1,
		AllowPrivateHosts: []string{"internal.test"},
	})

	response, err := client.Do(context.Background(), Request{
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

func TestClientRejectsPlainHTTPForPublicHost(t *testing.T) {
	t.Parallel()

	client := New(Config{
		Resolver: staticResolver{
			"api.example.test": {{
				IP: net.ParseIP("93.184.216.34"),
			}},
		},
		Timeout: 5 * time.Second,
	})

	_, err := client.Do(context.Background(), Request{
		Method: "GET",
		URL:    "http://api.example.test/resource",
	}, []string{"api.example.test"})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Do public http request error = %v, want ErrInvalidRequest", err)
	}
}

type staticResolver map[string][]net.IPAddr

func (r staticResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	items, ok := r[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host}
	}
	return items, nil
}

func testURLAndResolver(t *testing.T, rawURL string, host string) (string, Resolver) {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Parse URL %q: %v", rawURL, err)
	}
	listenerHost, _, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		t.Fatalf("SplitHostPort %q: %v", parsedURL.Host, err)
	}
	return parsedURL.Scheme + "://" + host + ":" + parsedURL.Port(), staticResolver{
		host: []net.IPAddr{{
			IP: net.ParseIP(listenerHost),
		}},
	}
}
