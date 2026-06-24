package renderstack

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

func TestPrepareBrowserPathKeepsConfiguredPath(t *testing.T) {
	original := resolveManagedBrowserPath
	t.Cleanup(func() {
		resolveManagedBrowserPath = original
	})

	called := false
	resolveManagedBrowserPath = func(context.Context, string) (string, error) {
		called = true
		return "", nil
	}

	got := prepareBrowserPath(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "  C:\\chromium\\chrome.exe  ")
	if got != "C:\\chromium\\chrome.exe" {
		t.Fatalf("prepareBrowserPath() = %q, want configured path", got)
	}
	if called {
		t.Fatal("expected configured browser path to bypass managed chromium bootstrap")
	}
}

func TestPrepareBrowserPathBootstrapsManagedChromium(t *testing.T) {
	original := resolveManagedBrowserPath
	t.Cleanup(func() {
		resolveManagedBrowserPath = original
	})

	resolveManagedBrowserPath = func(context.Context, string) (string, error) {
		return "C:\\managed\\chromium\\chrome.exe", nil
	}

	got := prepareBrowserPath(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "")
	if got != "C:\\managed\\chromium\\chrome.exe" {
		t.Fatalf("prepareBrowserPath() = %q, want managed chromium path", got)
	}
}

func TestPrepareBrowserPathLeavesDiagnosticsWhenBootstrapFails(t *testing.T) {
	original := resolveManagedBrowserPath
	t.Cleanup(func() {
		resolveManagedBrowserPath = original
	})

	resolveManagedBrowserPath = func(context.Context, string) (string, error) {
		return "", errors.New("bootstrap failed")
	}

	got := prepareBrowserPath(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "")
	if got != "" {
		t.Fatalf("prepareBrowserPath() = %q, want empty path on bootstrap failure", got)
	}
}
