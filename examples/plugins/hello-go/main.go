package main

import (
	"context"
	"os"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{PluginID: "hello-go", Subscriptions: []string{"message.group"}}, rayleabot.HandlerFunc(func(_ context.Context, event *rayleabot.EventContext) error {
		return event.Result(map[string]any{"handled": true, "summary": "hello-go accepted " + event.Event.EventType})
	}))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
