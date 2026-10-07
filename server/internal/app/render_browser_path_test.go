package app

import (
	"bytes"
	"context"
	"encoding/json"
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
		return "C:\\managed\\chromium\\chrome.exe", errors.New("bootstrap failed")
	}

	var logs bytes.Buffer
	got := prepareBrowserPath(context.Background(), slog.New(slog.NewJSONHandler(&logs, nil)), t.TempDir(), "", resolve)
	if got != "" {
		t.Fatalf("prepareBrowserPath() = %q, want empty path on bootstrap failure", got)
	}
	var diagnostic struct {
		Component string `json:"component"`
		Code      string `json:"code"`
	}
	if err := json.Unmarshal(logs.Bytes(), &diagnostic); err != nil {
		t.Fatal(err)
	}
	if diagnostic.Component != "render" || diagnostic.Code != "platform.resource_missing" {
		t.Fatalf("unexpected bootstrap diagnostic: %+v", diagnostic)
	}
}
