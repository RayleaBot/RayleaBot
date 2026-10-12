package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{}, rayleabot.HandlerFunc(handle))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	if event.Event.EventType == "config.changed" {
		return event.Result(map[string]any{"reloaded": true})
	}
	if event.Event.EventType == "management.action" {
		return handleNotes(ctx, event)
	}
	if event.Event.Command() != "config_panel" {
		return event.Result(map[string]any{"handled": false})
	}
	values := event.Config
	if args := event.Event.Args(); len(args) > 0 {
		city := strings.Join(args, " ")
		if _, err := event.Actions().ConfigWrite(ctx, map[string]any{"default_city": city}); err != nil {
			return err
		}
		values = map[string]any{"default_city": city, "unit": event.Config["unit"]}
	}
	return event.SendText(fmt.Sprintf("当前配置：城市=%v，单位=%v", values["default_city"], values["unit"]))
}

// notesMu serializes each read-modify-write of an account's notes.
var notesMu sync.Mutex

// handleNotes serves the bot_account "notes" page. The account comes from the
// host-validated event, never from the page's payload, and each account's notes
// live under their own key, so one account's page cannot read or overwrite another's.
func handleNotes(ctx context.Context, event *rayleabot.EventContext) error {
	action, _ := event.Event.Payload["action"].(string)
	if !strings.HasPrefix(action, "notes.") {
		return event.Fail("platform.invalid_request", "未知的管理动作")
	}
	bot, ok := event.Event.ManagementBot()
	if !ok {
		return event.Fail("platform.invalid_request", "账号备注需要在按账号的页面中操作")
	}
	input, _ := event.Event.Payload["payload"].(map[string]any)
	key := "notes:" + bot.SourceProtocol + ":" + bot.BotID

	notesMu.Lock()
	defer notesMu.Unlock()
	notes, err := readNotes(ctx, event, key)
	if err != nil {
		return err
	}
	switch action {
	case "notes.list":
	case "notes.add":
		text, _ := input["text"].(string)
		if text = strings.TrimSpace(text); text == "" || len([]rune(text)) > 200 {
			return event.Fail("platform.invalid_request", "备注需要 1 至 200 个字符")
		}
		notes = append(notes, text)
	case "notes.remove":
		index, ok := input["index"].(float64)
		if !ok || index < 0 || int(index) >= len(notes) || index != float64(int(index)) {
			return event.Fail("platform.invalid_request", "要删除的备注不存在")
		}
		notes = append(notes[:int(index)], notes[int(index)+1:]...)
	default:
		return event.Fail("platform.invalid_request", "未知的管理动作")
	}
	if action != "notes.list" {
		if _, err := event.Actions().KVSet(ctx, key, notes); err != nil {
			return err
		}
	}
	return event.Result(map[string]any{"notes": notes})
}

func readNotes(ctx context.Context, event *rayleabot.EventContext, key string) ([]string, error) {
	result, err := event.Actions().KVGet(ctx, key)
	if err != nil {
		return nil, err
	}
	notes := []string{}
	if exists, _ := result["exists"].(bool); !exists {
		return notes, nil
	}
	raw, err := json.Marshal(result["value"])
	if err != nil {
		return nil, err
	}
	return notes, json.Unmarshal(raw, &notes)
}
