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
		if event.Config["fixture_acceptance"] == true && event.Event.EventType == "scheduler.trigger" {
			return acceptanceSchedulerProbe(ctx, event)
		}
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
	fields := map[string]any{
		"acceptance_probe": probe, "fixture_pid": os.Getpid(),
		"artifact_id": rendered["artifact_id"], "mime": rendered["mime"],
	}
	if event.Config["acceptance_schedule"] == true {
		jobID := "raylea.echo.acceptance." + probe
		scheduled, err := event.Actions().SchedulerCreate(ctx, rayleabot.SchedulerCreateRequest{
			TaskID: jobID, Cron: "* * * * *", LogLabel: "Release acceptance",
			Payload: map[string]any{"acceptance_probe": probe, "job_id": jobID},
		})
		if err != nil {
			return fmt.Errorf("acceptance schedule: %w", err)
		}
		if scheduled["task_id"] != jobID {
			return fmt.Errorf("acceptance scheduler returned an unexpected task identity")
		}
		fields["scheduler_job_id"] = jobID
	}
	_, err = event.Actions().LoggerWrite(ctx, rayleabot.LoggerWriteRequest{
		Level: "info", Message: "Release acceptance render completed",
		Fields: fields,
	})
	if err != nil {
		return err
	}
	return event.Result(map[string]any{"handled": true, "acceptance_probe": probe})
}

func acceptanceSchedulerProbe(ctx context.Context, event *rayleabot.EventContext) error {
	payload, _ := event.Event.Payload["payload"].(map[string]any)
	probe, _ := payload["acceptance_probe"].(string)
	jobID, _ := payload["job_id"].(string)
	if probe == "" || probe != event.Config["acceptance_probe"] || jobID != "raylea.echo.acceptance."+probe {
		return fmt.Errorf("acceptance scheduler payload does not identify this probe")
	}
	_, err := event.Actions().LoggerWrite(ctx, rayleabot.LoggerWriteRequest{
		Level: "info", Message: "Release acceptance scheduled event completed",
		Fields: map[string]any{
			"acceptance_scheduler_probe": probe, "scheduler_job_id": jobID, "fixture_pid": os.Getpid(),
			"event_type": event.Event.EventType, "source_protocol": event.Event.SourceProtocol,
			"source_adapter": event.Event.SourceAdapter,
		},
	})
	if err != nil {
		return err
	}
	return event.Result(map[string]any{"handled": true, "acceptance_scheduler_probe": probe})
}
