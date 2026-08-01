package main

import (
	"context"
	"os"
	"strings"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() { run("echo-go", []string{"message.group", "message.private"}, handle) }

func handle(_ context.Context, event *rayleabot.EventContext) error {
	if event.Event.Command() != "echo" && event.Event.Command() != "repeat" {
		return event.Result(map[string]any{"handled": false})
	}
	text := strings.TrimSpace(strings.Join(event.Event.Args(), " "))
	if text == "" {
		text = strings.TrimSpace(event.Event.Message.PlainText)
	}
	if text == "" {
		text = "（空消息）"
	}
	return event.SendText(text)
}

func run(pluginID string, subscriptions []string, handler rayleabot.HandlerFunc) {
	if err := rayleabot.Run(context.Background(), rayleabot.Options{PluginID: pluginID, Subscriptions: subscriptions}, handler); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}
