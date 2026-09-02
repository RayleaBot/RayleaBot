package main

import (
	"context"
	"os"
	"sort"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{}, rayleabot.HandlerFunc(handle))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	if event.Event.EventType == "webhook.received" {
		keys := make([]string, 0, len(event.Event.Payload))
		for key := range event.Event.Payload {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		route := event.Event.Webhook.Route
		_, _ = event.Actions().LoggerWrite(ctx, rayleabot.LoggerWriteRequest{Level: "info", Message: "Webhook route " + route + " was received and accepted for processing.", Fields: map[string]any{"route": route}})
		return event.Result(map[string]any{"handled": true, "raw_payload_keys": keys})
	}
	return event.Result(map[string]any{"handled": false})
}
