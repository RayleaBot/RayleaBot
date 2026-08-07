package main

import (
	"context"
	"os"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{PluginID: "example-scheduler", Subscriptions: []string{"message.group", "scheduler.trigger"}}, rayleabot.HandlerFunc(handle))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	if event.Event.EventType == "scheduler.trigger" {
		return event.Result(map[string]any{"handled": true, "payload": event.Event.Payload})
	}
	if event.Event.Command() != "schedule_daily" {
		return event.Result(map[string]any{"handled": false})
	}
	result, err := event.Actions().SchedulerCreate(ctx, rayleabot.SchedulerCreateRequest{TaskID: "daily_morning_report", Cron: "0 8 * * *", EventType: "scheduler.trigger", LogLabel: "每日早报", Payload: map[string]any{"report_type": "daily"}})
	if err != nil {
		return err
	}
	return event.Result(map[string]any{"scheduled": result})
}
