package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{PluginID: "example-config-panel", Subscriptions: []string{"message.group", "message.private", "config.changed"}}, rayleabot.HandlerFunc(handle))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	if event.Event.EventType == "config.changed" {
		return event.Result(map[string]any{"reloaded": true})
	}
	if event.Event.Command() != "config_panel" {
		return event.Result(map[string]any{"handled": false})
	}
	if args := event.Event.Args(); len(args) > 0 {
		if _, err := event.Actions().ConfigWrite(ctx, map[string]any{"default_city": strings.Join(args, " ")}); err != nil {
			return err
		}
	}
	result, err := event.Actions().ConfigRead(ctx, "default_city", "unit")
	if err != nil {
		return err
	}
	values, _ := result["values"].(map[string]any)
	return event.SendText(fmt.Sprintf("当前配置：城市=%v，单位=%v", values["default_city"], values["unit"]))
}
