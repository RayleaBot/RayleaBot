package main

import (
	"context"
	"os"
	"strings"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{PluginID: "example-plugin-list", Subscriptions: []string{"message.group", "message.private"}}, rayleabot.HandlerFunc(handle))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	if event.Event.Command() != "plugins_demo" {
		return event.Result(map[string]any{"handled": false})
	}
	result, err := event.Actions().PluginList(ctx)
	if err != nil {
		return err
	}
	items, _ := result["items"].([]any)
	names := make([]string, 0, len(items))
	for _, raw := range items {
		if item, ok := raw.(map[string]any); ok {
			name, _ := item["name"].(string)
			if name == "" {
				name, _ = item["id"].(string)
			}
			if name != "" {
				names = append(names, name)
			}
		}
	}
	if len(names) == 0 {
		return event.SendText("当前没有插件。")
	}
	return event.SendText("已加载插件：\n" + strings.Join(names, "\n"))
}
