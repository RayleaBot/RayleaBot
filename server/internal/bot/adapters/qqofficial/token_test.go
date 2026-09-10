package qqofficial

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenSourceCachesUntilTheRenewWindow(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "token-" + string(rune('a'+calls-1)), "expires_in": 7200})
	}))
	defer server.Close()

	now := time.Unix(1_700_000_000, 0)
	source := NewTokenSource("app", "secret", server.Client())
	source.now = func() time.Time { return now }
	source.endpoint = server.URL

	first, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("first token: %v", err)
	}
	// Well inside the lifetime: served from cache, no second request.
	now = now.Add(7000 * time.Second)
	second, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("cached token: %v", err)
	}
	if second != first || calls != 1 {
		t.Fatalf("token = %q after %d calls, want the cached %q after 1", second, calls, first)
	}

	// Inside the 60s renew lead time: refreshed before it can expire mid-request.
	now = now.Add(150 * time.Second)
	third, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("renewed token: %v", err)
	}
	if third == first || calls != 2 {
		t.Fatalf("token = %q after %d calls, want a renewed value after 2", third, calls)
	}
}

func TestTokenSourceKeepsServingWhileRefreshFails(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "good", "expires_in": 7200})
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "upstream unavailable"})
	}))
	defer server.Close()

	now := time.Unix(1_700_000_000, 0)
	source := NewTokenSource("app", "secret", server.Client())
	source.now = func() time.Time { return now }
	source.endpoint = server.URL

	if _, err := source.Token(context.Background()); err != nil {
		t.Fatalf("first token: %v", err)
	}
	// Refresh fails while the cached token is still valid: keep serving it
	// rather than dropping the connection over a transient upstream error.
	now = now.Add(7180 * time.Second)
	token, err := source.Token(context.Background())
	if err != nil || token != "good" {
		t.Fatalf("token = %q, err = %v; want the cached token to survive a failed refresh", token, err)
	}

	// Past expiry the stale token is worthless, so the failure must surface.
	now = now.Add(200 * time.Second)
	if _, err := source.Token(context.Background()); err == nil {
		t.Fatal("expected an error once the cached token has expired")
	}
}

func TestParseExpiresInAcceptsBothSpellings(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"number", `7200`, 7200 * time.Second},
		{"string", `"7200"`, 7200 * time.Second},
		{"absent", ``, 7200 * time.Second},
		{"unparseable", `"soon"`, 7200 * time.Second},
		{"zero", `0`, 7200 * time.Second},
	} {
		if got := parseExpiresIn(json.RawMessage(tc.raw)); got != tc.want {
			t.Fatalf("%s: parseExpiresIn(%q) = %v, want %v", tc.name, tc.raw, got, tc.want)
		}
	}
}
