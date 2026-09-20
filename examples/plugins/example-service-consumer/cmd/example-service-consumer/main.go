package main

import (
	"context"
	"fmt"
	"log"
	"time"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	options := rayleabot.Options{Services: []rayleabot.Service{{Name: "resource", Version: 1, Methods: map[string]rayleabot.ServiceHandler{
		"query": func(_ context.Context, event *rayleabot.EventContext, request rayleabot.ServiceRequest) (map[string]any, error) {
			return map[string]any{"owner": event.PluginID, "input": request.Params}, nil
		},
	}}}}
	if err := rayleabot.Run(context.Background(), options, rayleabot.HandlerFunc(handle)); err != nil {
		log.Fatal(err)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	if event.Event.EventType != "management.action" && event.Event.Command() != "服务调用示例" {
		return nil
	}
	provider, _ := event.Config["provider"].(string)
	if provider == "" {
		provider = "example-service-provider"
	}
	method := "query"
	params := map[string]any{"id": "item-1"}
	if data, ok := event.Event.Payload["payload"].(map[string]any); ok {
		if value, ok := data["method"].(string); ok && value != "" {
			method = value
		}
		if value, ok := data["params"].(map[string]any); ok {
			params = value
		}
		if milliseconds, ok := data["timeout_ms"].(float64); ok && milliseconds > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(milliseconds)*time.Millisecond)
			defer cancel()
		}
	}
	var result map[string]any
	if _, err := event.Actions().KVSet(ctx, "last_requested_id", params["id"]); err != nil {
		return err
	}
	err := event.Actions().CallService(ctx, rayleabot.ServiceCallRequest{TargetPluginID: provider, Service: "resource", ServiceVersion: 1, Method: method, Params: params}, &result)
	if err != nil {
		if failure, ok := err.(*rayleabot.ActionError); ok {
			return event.FailDetails(failure.Code, failure.Message, failure.Details)
		}
		return event.Fail("plugin.event_canceled", "service call canceled")
	}
	if event.Event.Command() == "服务调用示例" {
		return event.SendText(fmt.Sprintf("服务返回：%v", result["id"]))
	}
	return event.Result(result)
}
