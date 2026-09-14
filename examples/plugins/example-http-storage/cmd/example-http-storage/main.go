package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

const maxResponseBytes = 4 << 20

var client = &http.Client{Timeout: 10 * time.Second}

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{}, rayleabot.HandlerFunc(handle))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	if event.Event.Command() != "scope_fetch" && event.Event.Command() != "scope_cache" {
		return event.Result(map[string]any{"handled": false})
	}
	dataDir := os.Getenv("RAYLEABOT_PLUGIN_DATA_DIR")
	if dataDir == "" {
		return errors.New("RAYLEABOT_PLUGIN_DATA_DIR is not set")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com/", nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return err
	}
	path := filepath.Join(dataDir, "cache", "example.html")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return err
	}
	_, _ = event.Actions().LoggerWrite(ctx, rayleabot.LoggerWriteRequest{Level: "info", Message: "The HTTP response was cached in the plugin data directory.", Fields: map[string]any{"status_code": response.StatusCode, "cached_path": "cache/example.html"}})
	return event.Result(map[string]any{"handled": true, "cached_path": "cache/example.html"})
}
