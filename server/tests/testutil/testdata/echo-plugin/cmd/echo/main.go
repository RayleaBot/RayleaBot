package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{}, rayleabot.HandlerFunc(func(ctx context.Context, event *rayleabot.EventContext) error {
		if event.Config["fixture_acceptance"] == true && event.Event.EventType == "config.changed" {
			return acceptanceProbe(ctx, event)
		}
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

// This path is opt-in test-fixture behavior, exercised through the normal SDK.
func acceptanceProbe(ctx context.Context, event *rayleabot.EventContext) error {
	probe, _ := event.Config["acceptance_probe"].(string)
	if strings.TrimSpace(probe) == "" {
		return event.Result(map[string]any{"handled": false})
	}
	rendered, err := event.Actions().RenderImage(ctx, rayleabot.RenderImageRequest{
		Template: "help.menu", Theme: "default", Output: "png",
		Data: map[string]any{
			"title": "Release acceptance " + probe,
			"items": []map[string]any{{"name": "fixture", "description": "Native SDK render acceptance"}},
		},
	})
	if err != nil {
		return fmt.Errorf("acceptance render: %w", err)
	}
	_, err = event.Actions().LoggerWrite(ctx, rayleabot.LoggerWriteRequest{
		Level: "info", Message: "Release acceptance render completed",
		Fields: map[string]any{
			"acceptance_probe": probe, "fixture_pid": os.Getpid(),
			"artifact_id": rendered["artifact_id"], "mime": rendered["mime"],
		},
	})
	if err != nil {
		return err
	}
	return event.Result(map[string]any{"handled": true, "acceptance_probe": probe})
}
