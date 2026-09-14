package main

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"time"
	"unicode/utf8"

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
	path := "cache/example.html"
	if utf8.Valid(body) {
		_, err = event.Actions().FileWriteText(ctx, path, string(body))
	} else {
		path = "cache/example.bin"
		_, err = event.Actions().FileWriteBase64(ctx, path, base64.StdEncoding.EncodeToString(body))
	}
	if err != nil {
		return err
	}
	_, _ = event.Actions().LoggerWrite(ctx, rayleabot.LoggerWriteRequest{Level: "info", Message: "The HTTP response was cached at " + path + ".", Fields: map[string]any{"status_code": response.StatusCode, "cached_path": path}})
	return event.Result(map[string]any{"handled": true, "cached_path": path})
}
