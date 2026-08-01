package main

import (
	"context"
	"os"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{PluginID: "notice-logger", Subscriptions: []string{"notice.member_increase", "notice.member_decrease"}}, rayleabot.HandlerFunc(handle))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	key := "notice:member_decrease:count"
	message := "member left notice received"
	if event.Event.EventType == "notice.member_increase" {
		key = "notice:member_increase:count"
		message = "member joined notice received"
	}
	_, _ = event.Actions().LoggerWrite(ctx, rayleabot.LoggerWriteRequest{Level: "info", Message: message, Fields: map[string]any{"user_id": event.Event.Actor.ID, "group_id": event.Event.Target.ID, "sub_type": event.Event.Payload["sub_type"]}})
	result, _ := event.Actions().KVGet(ctx, key)
	count := float64(0)
	if value, ok := result["value"].(float64); ok {
		count = value
	}
	if _, err := event.Actions().KVSet(ctx, key, count+1); err != nil {
		return err
	}
	return event.Result(map[string]any{"logged": true})
}
