package main

import (
	"context"
	"os"
	"strings"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{}, rayleabot.HandlerFunc(func(ctx context.Context, event *rayleabot.EventContext) error {
		if event.Config["fixture_composition"] == true {
			if args := event.Event.Args(); len(args) == 2 && args[0] == "write" {
				result, err := event.Actions().ConfigWrite(ctx, map[string]any{"value": args[1]})
				if err != nil {
					return err
				}
				return event.Result(result)
			}
			_, err := event.Actions().LoggerWrite(ctx, rayleabot.LoggerWriteRequest{
				Level: "info", Message: "Composition fixture observed its settings",
				Fields: map[string]any{"fixture_event": event.Event.EventType, "fixture_value": event.Config["value"]},
			})
			if err != nil {
				return err
			}
			return event.Result(map[string]any{"handled": true})
		}
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
