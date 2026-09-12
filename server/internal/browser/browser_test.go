package browser

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestValidateRemoteDebuggingURL(t *testing.T) {
	t.Parallel()

	valid := []string{
		"http://127.0.0.1:9222",
		"http://localhost:9222",
		"https://[::1]:9222",
		"ws://127.0.0.1:9222/devtools/browser/fixture",
		"wss://localhost/devtools/browser/fixture",
	}
	for _, raw := range valid {
		if err := validateRemoteDebuggingURL(raw); err != nil {
			t.Fatalf("validateRemoteDebuggingURL(%q) = %v, want nil", raw, err)
		}
	}

	invalid := map[string]string{
		"public host":  "https://example.com:9222",
		"credentials":  "http://user:password@127.0.0.1:9222",
		"bad scheme":   "ftp://127.0.0.1:9222",
		"missing host": "http://",
		"fragment":     "http://127.0.0.1:9222#fragment",
	}
	for name, raw := range invalid {
		if err := validateRemoteDebuggingURL(raw); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("%s: validateRemoteDebuggingURL(%q) = %v, want ErrInvalidRequest", name, raw, err)
		}
	}
}

func TestResolveRemoteDebuggingURLAcceptsDirectWebSocketEndpoint(t *testing.T) {
	t.Parallel()

	raw := "ws://127.0.0.1:9222/devtools/browser/fixture"
	got, err := resolveRemoteDebuggingURL(context.Background(), raw)
	if err != nil || got != raw {
		t.Fatalf("resolveRemoteDebuggingURL(%q) = %q, %v", raw, got, err)
	}
}

func TestResolveRemoteDebuggingURLRejectsDiscoveryForNonLoopbackWebSocket(t *testing.T) {
	t.Parallel()

	_, err := resolveRemoteDebuggingURL(context.Background(), "ws://192.0.2.10:9222/devtools/browser/fixture")
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("non-loopback direct endpoint error = %v, want ErrInvalidRequest", err)
	}
}

func TestResolveBrowserPathPreference(t *testing.T) {
	t.Parallel()

	exists := func(path string) bool { return path == "configured" || path == "managed" || path == "system" }
	if got := resolveBrowserPathWith("configured", "managed", func() string { return "system" }, exists); got != "configured" {
		t.Fatalf("configured path = %q", got)
	}
	if got := resolveBrowserPathWith("missing", "managed", func() string { return "system" }, exists); got != "managed" {
		t.Fatalf("managed fallback = %q", got)
	}
	if got := resolveBrowserPathWith("", "", func() string { return "system" }, exists); got != "system" {
		t.Fatalf("system fallback = %q", got)
	}
	if got := resolveBrowserPathWith("", "", func() string { return "" }, exists); got != "" {
		t.Fatalf("missing browser = %q", got)
	}
}

func TestBrowserLaunchArgsFilterReservedFlags(t *testing.T) {
	t.Parallel()

	args := browserLaunchArgs(launchAttempt{mode: ModeHeadless}, []string{
		"--user-data-dir=C:/other",
		"--remote-debugging-port=9222",
		"--headless",
		"--disable-features=Custom",
		"--custom-flag=value",
	}, "C:/profile", 9333)

	joined := strings.Join(args, " ")
	for _, forbidden := range []string{"C:/other", "--remote-debugging-port=9222", "--headless "} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("reserved flag %q survived: %v", forbidden, args)
		}
	}
	if !strings.Contains(joined, "--custom-flag=value") {
		t.Fatalf("configured flag missing: %v", args)
	}
	if !strings.Contains(joined, "--user-data-dir=C:/profile") || !strings.Contains(joined, "--remote-debugging-port=9333") {
		t.Fatalf("host-owned flags missing: %v", args)
	}
	if !strings.Contains(joined, "--headless=new") {
		t.Fatalf("headless flag missing: %v", args)
	}
}

func TestManagerProfileExclusivity(t *testing.T) {
	t.Parallel()
	manager := NewManager(Options{ProfileRoot: t.TempDir()})
	defer manager.CloseAll()
	req := LaunchRequest{Profile: "default", Mode: ModeRemoteCDP, RemoteDebuggingURL: "ws://127.0.0.1:9222/devtools/browser/fixture"}
	first, err := manager.Launch(context.Background(), "weather", req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Launch(context.Background(), "weather", req); !errors.Is(err, ErrBusy) {
		t.Fatalf("same profile: %v", err)
	}
	if _, err = manager.Launch(context.Background(), "other", req); err != nil {
		t.Fatal(err)
	}
	if !manager.Close("weather", first.ID) {
		t.Fatal("close failed")
	}
	if _, err = manager.Launch(context.Background(), "weather", req); err != nil {
		t.Fatal(err)
	}
}

func TestManagerLaunchRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	manager := NewManager(Options{ProfileRoot: t.TempDir()})
	for name, request := range map[string]LaunchRequest{
		"invalid profile": {Profile: "Bad Profile"},
		"invalid mode":    {Profile: "default", Mode: "personal"},
		"missing remote":  {Profile: "default", Mode: ModeRemoteCDP},
	} {
		if _, err := manager.Launch(context.Background(), "weather", request); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("%s: Launch error = %v, want ErrInvalidRequest", name, err)
		}
	}
	if _, err := manager.Launch(context.Background(), "", LaunchRequest{Profile: "default"}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatal("missing plugin id was accepted")
	}
}

func TestManagerCloseUnknownSession(t *testing.T) {
	t.Parallel()

	manager := NewManager(Options{})
	if manager.Close("weather", "missing") {
		t.Fatal("closing an unknown session reported success")
	}
}

func TestLocalLaunchRequiresBrowserPath(t *testing.T) {
	t.Parallel()

	if _, _, err := launchLocalBrowser(context.Background(), Options{ProfileRoot: t.TempDir()}, "weather", "default", launchAttempt{mode: ModeHeadless, useProfile: true}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("launchLocalBrowser error = %v, want ErrUnavailable", err)
	}
}
