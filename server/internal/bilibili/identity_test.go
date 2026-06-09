package bilibili

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewIdentityProvider(t *testing.T) {
	t.Parallel()
	p := NewIdentityProvider(nil)
	if p == nil {
		t.Fatal("NewIdentityProvider returned nil")
	}
	ua := p.UserAgent()
	if ua == "" {
		t.Fatal("UserAgent returned empty string")
	}
}

func TestUserAgentRotates(t *testing.T) {
	t.Parallel()
	p := NewIdentityProvider(nil)
	first := p.UserAgent()
	// Collect several UAs and verify at least one differs.
	found := false
	for i := 0; i < 20; i++ {
		if p.UserAgent() != first {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("UserAgent did not rotate after multiple calls")
	}
}

func TestWithFixedUA(t *testing.T) {
	t.Parallel()
	p := NewIdentityProvider(nil)
	fixed := "test-ua/1.0"
	p.WithFixedUA(fixed)
	for i := 0; i < 5; i++ {
		if got := p.UserAgent(); got != fixed {
			t.Fatalf("WithFixedUA: got %q, want %q", got, fixed)
		}
	}
}

func TestApplyHeadersSetsRequiredKeys(t *testing.T) {
	t.Parallel()
	p := NewIdentityProvider(nil)
	req, _ := http.NewRequest(http.MethodGet, "https://api.bilibili.com/test", nil)
	p.ApplyHeaders(req, http.MethodGet)

	required := []string{"Accept", "Accept-Language", "User-Agent", "Referer", "Origin", "Sec-Fetch-Dest", "Sec-Fetch-Mode", "Sec-Fetch-Site"}
	for _, key := range required {
		if req.Header.Get(key) == "" {
			t.Fatalf("ApplyHeaders missing header %q", key)
		}
	}
	if req.Header.Get("Referer") != "https://www.bilibili.com/" {
		t.Fatalf("ApplyHeaders Referer = %q, want https://www.bilibili.com/", req.Header.Get("Referer"))
	}
	if req.Header.Get("Origin") != "https://www.bilibili.com" {
		t.Fatalf("ApplyHeaders Origin = %q, want https://www.bilibili.com", req.Header.Get("Origin"))
	}
}

func TestApplyLiveHeadersSetsLiveRefererOrigin(t *testing.T) {
	t.Parallel()
	p := NewIdentityProvider(nil)
	req, _ := http.NewRequest(http.MethodGet, "https://api.live.bilibili.com/test", nil)
	p.ApplyLiveHeaders(req, http.MethodGet)

	if req.Header.Get("Referer") != "https://live.bilibili.com/" {
		t.Fatalf("ApplyLiveHeaders Referer = %q, want https://live.bilibili.com/", req.Header.Get("Referer"))
	}
	if req.Header.Get("Origin") != "https://live.bilibili.com" {
		t.Fatalf("ApplyLiveHeaders Origin = %q, want https://live.bilibili.com", req.Header.Get("Origin"))
	}
	required := []string{"Accept", "Accept-Language", "User-Agent", "Sec-Fetch-Dest", "Sec-Fetch-Mode", "Sec-Fetch-Site"}
	for _, key := range required {
		if req.Header.Get(key) == "" {
			t.Fatalf("ApplyLiveHeaders missing header %q", key)
		}
	}
}

func TestJitteredDelayRange(t *testing.T) {
	t.Parallel()
	p := NewIdentityProvider(func() time.Time {
		return time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	})
	base := 60 * time.Second
	min := time.Duration(float64(base) * 0.7)
	max := time.Duration(float64(base) * 1.3)
	for i := 0; i < 100; i++ {
		got := p.JitteredDelay(base)
		if got < min || got >= max {
			t.Fatalf("JitteredDelay(%v) = %v, want [%v, %v)", base, got, min, max)
		}
	}
}

func TestJitteredDelayZero(t *testing.T) {
	t.Parallel()
	p := NewIdentityProvider(nil)
	if got := p.JitteredDelay(0); got != 0 {
		t.Fatalf("JitteredDelay(0) = %v, want 0", got)
	}
	if got := p.JitteredDelay(-1); got != 0 {
		t.Fatalf("JitteredDelay(-1) = %v, want 0", got)
	}
}

func TestApplyHeadersContentTypeForPost(t *testing.T) {
	t.Parallel()
	p := NewIdentityProvider(nil)
	req, _ := http.NewRequest(http.MethodPost, "https://api.bilibili.com/test", strings.NewReader("test"))
	p.ApplyHeaders(req, http.MethodPost)
	if ct := req.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
		t.Fatalf("ApplyHeaders POST Content-Type = %q, want application/x-www-form-urlencoded", ct)
	}
}
