package main

import (
	"context"
	"os"
	"strings"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{
		PluginID:      "raylea.echo",
		Subscriptions: []string{"message.group", "message.private"},
	}, rayleabot.HandlerFunc(func(_ context.Context, event *rayleabot.EventContext) error {
		if event.Event.Command() != "echo" {
			return event.Result(map[string]any{"handled": false})
		}
		text := strings.TrimSpace(strings.Join(event.Event.Args(), " "))
		if text == "" {
			text = strings.TrimSpace(event.Event.Message.PlainText)
		}
		return event.SendText(text)
	}))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
