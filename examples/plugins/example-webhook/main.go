package main

import (
	"context"
	"os"
	"sort"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{PluginID: "example-webhook", Subscriptions: []string{"message.group", "webhook.received"}}, rayleabot.HandlerFunc(handle))
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
		_, _ = event.Actions().LoggerWrite(ctx, rayleabot.LoggerWriteRequest{Level: "info", Message: "webhook received", Fields: map[string]any{"route": event.Event.Target.ID}})
		return event.Result(map[string]any{"handled": true, "raw_payload_keys": keys})
	}
	if event.Event.Command() != "webhook_register" {
		return event.Result(map[string]any{"handled": false})
	}
	result, err := event.Actions().ExposeWebhook(ctx, rayleabot.ExposeWebhookRequest{Route: "github", SecretRef: "webhook.github.secret", AuthStrategy: "hmac_sha256", Header: "X-Hub-Signature-256", SignaturePrefix: "sha256="})
	if err != nil {
		return err
	}
	return event.Result(map[string]any{"webhook": result})
}
