package main

import (
	"context"
	"log"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	service := rayleabot.Service{Name: "resource", Version: 1, Methods: map[string]rayleabot.ServiceHandler{
		"query": query,
		"wait": func(ctx context.Context, event *rayleabot.EventContext, request rayleabot.ServiceRequest) (map[string]any, error) {
			if _, err := event.Actions().KVSet(ctx, "waiting", true); err != nil {
				return nil, err
			}
			<-ctx.Done()
			return nil, ctx.Err()
		},
		"fail": func(context.Context, *rayleabot.EventContext, rayleabot.ServiceRequest) (map[string]any, error) {
			return nil, &rayleabot.ActionError{Code: "plugin.service_unavailable", Message: "example resource unavailable", Details: map[string]any{"reason": "fixture"}}
		},
		"nested": func(ctx context.Context, event *rayleabot.EventContext, request rayleabot.ServiceRequest) (map[string]any, error) {
			var result map[string]any
			err := event.Actions().CallService(ctx, rayleabot.ServiceCallRequest{TargetPluginID: request.CallerPluginID, Service: "resource", ServiceVersion: 1, Method: "query", Params: map[string]any{}}, &result)
			return result, err
		},
	}}
	if err := rayleabot.Run(context.Background(), rayleabot.Options{Services: []rayleabot.Service{service}}, nil); err != nil {
		log.Fatal(err)
	}
}

func query(ctx context.Context, event *rayleabot.EventContext, request rayleabot.ServiceRequest) (map[string]any, error) {
	id, _ := request.Params["id"].(string)
	if _, err := event.Actions().KVSet(ctx, "last_id", id); err != nil {
		return nil, err
	}
	result := map[string]any{"id": id, "caller": request.CallerPluginID, "bot_id": request.Origin.BotID, "input": request.Params}
	if request.Origin.Actor != nil {
		result["actor_id"] = request.Origin.Actor.ID
	}
	return result, nil
}
