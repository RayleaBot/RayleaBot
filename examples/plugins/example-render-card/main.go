package main

import (
	"context"
	"os"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{PluginID: "example-render-card", Subscriptions: []string{"message.group", "message.private"}}, rayleabot.HandlerFunc(handle))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	if event.Event.Command() != "render_card" {
		return event.Result(map[string]any{"handled": false})
	}
	result, err := event.Actions().RenderImage(ctx, rayleabot.RenderImageRequest{Template: "card", Theme: "default", Output: "png", FallbackText: "Render unavailable.", Data: map[string]any{"title": "Render Example", "items": []map[string]string{{"name": "weather", "description": "Query weather"}, {"name": "echo", "description": "Repeat text"}}}})
	if err != nil {
		return err
	}
	imagePath, _ := result["image_path"].(string)
	if imagePath == "" {
		return event.SendText("Render unavailable.")
	}
	return event.Send(event.Event.Target.Type, event.Event.Target.ID, rayleabot.Image(imagePath))
}
