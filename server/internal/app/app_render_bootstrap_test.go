package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

func TestPrepareRenderBrowserPathKeepsConfiguredPath(t *testing.T) {
	original := resolveManagedRenderBrowserPath
	t.Cleanup(func() {
		resolveManagedRenderBrowserPath = original
	})

	called := false
	resolveManagedRenderBrowserPath = func(context.Context, string) (string, error) {
		called = true
		return "", nil
	}

	got := prepareRenderBrowserPath(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "  C:\\chromium\\chrome.exe  ")
	if got != "C:\\chromium\\chrome.exe" {
		t.Fatalf("prepareRenderBrowserPath() = %q, want configured path", got)
	}
	if called {
		t.Fatal("expected configured browser path to bypass managed chromium bootstrap")
	}
}

func TestPrepareRenderBrowserPathBootstrapsManagedChromium(t *testing.T) {
	original := resolveManagedRenderBrowserPath
	t.Cleanup(func() {
		resolveManagedRenderBrowserPath = original
	})

	resolveManagedRenderBrowserPath = func(context.Context, string) (string, error) {
		return "C:\\managed\\chromium\\chrome.exe", nil
	}

	got := prepareRenderBrowserPath(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "")
	if got != "C:\\managed\\chromium\\chrome.exe" {
		t.Fatalf("prepareRenderBrowserPath() = %q, want managed chromium path", got)
	}
}

func TestPrepareRenderBrowserPathLeavesDiagnosticsWhenBootstrapFails(t *testing.T) {
	original := resolveManagedRenderBrowserPath
	t.Cleanup(func() {
		resolveManagedRenderBrowserPath = original
	})

	resolveManagedRenderBrowserPath = func(context.Context, string) (string, error) {
		return "", errors.New("bootstrap failed")
	}

	got := prepareRenderBrowserPath(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)), t.TempDir(), "")
	if got != "" {
		t.Fatalf("prepareRenderBrowserPath() = %q, want empty path on bootstrap failure", got)
	}
}
