package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

func TestPrepareBrowserPathKeepsConfiguredPath(t *testing.T) {
	t.Parallel()

	called := false
	resolve := func(context.Context, string) (string, error) {
		called = true
		return "", nil
	}

	got := prepareBrowserPath(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "  C:\\chromium\\chrome.exe  ", resolve)
	if got != "C:\\chromium\\chrome.exe" {
		t.Fatalf("prepareBrowserPath() = %q, want configured path", got)
	}
	if called {
		t.Fatal("expected configured browser path to bypass managed chromium bootstrap")
	}
}

func TestPrepareBrowserPathBootstrapsManagedChromium(t *testing.T) {
	t.Parallel()

	resolve := func(context.Context, string) (string, error) {
		return "C:\\managed\\chromium\\chrome.exe", nil
	}

	got := prepareBrowserPath(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "", resolve)
	if got != "C:\\managed\\chromium\\chrome.exe" {
		t.Fatalf("prepareBrowserPath() = %q, want managed chromium path", got)
	}
}

func TestPrepareBrowserPathLeavesDiagnosticsWhenBootstrapFails(t *testing.T) {
	t.Parallel()

	resolve := func(context.Context, string) (string, error) {
		return "", errors.New("bootstrap failed")
	}

	got := prepareBrowserPath(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "", resolve)
	if got != "" {
		t.Fatalf("prepareBrowserPath() = %q, want empty path on bootstrap failure", got)
	}
}
