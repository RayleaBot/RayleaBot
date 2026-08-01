package main

import (
	"context"
	"os"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{PluginID: "example-capability-parameters", Subscriptions: []string{"message.group", "message.private"}, MaxConcurrentHandlers: 2}, rayleabot.HandlerFunc(handle))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	if event.Event.Command() != "scope_fetch" && event.Event.Command() != "scope_cache" {
		return event.Result(map[string]any{"handled": false})
	}
	result, err := event.Actions().HTTPRequest(ctx, rayleabot.HTTPRequest{Method: "GET", URL: "https://example.com/"})
	if err != nil {
		return err
	}
	path := "cache/example.html"
	if body, ok := result["body_text"].(string); ok {
		_, err = event.Actions().FileWriteText(ctx, path, body)
	} else {
		path = "cache/example.bin"
		body, _ := result["body_base64"].(string)
		_, err = event.Actions().FileWriteBase64(ctx, path, body)
	}
	if err != nil {
		return err
	}
	_, _ = event.Actions().LoggerWrite(ctx, rayleabot.LoggerWriteRequest{Level: "info", Message: "scoped content cached", Fields: map[string]any{"status_code": result["status_code"], "cached_path": path}})
	return event.Result(map[string]any{"handled": true, "cached_path": path})
}
