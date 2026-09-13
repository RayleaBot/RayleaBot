package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	if err := rayleabot.Run(context.Background(), rayleabot.Options{}, rayleabot.HandlerFunc(handle)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type selection struct {
	Role      string `json:"role"`
	Operation string `json:"operation"`
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	if event.Event.EventType == "session.expired" {
		return sendText(ctx, event, "会话已超时，请重新发送 /会话演示。")
	}
	if event.Event.Command() != "会话演示" {
		return event.Fail("plugin.not_handled", "not handled")
	}
	draft := &selection{}
	notify := true
	options := rayleabot.SessionWaitOptions{NotifyOnExpire: &notify}
	var role, operation, confirm rayleabot.HandlerFunc
	role = func(ctx context.Context, next *rayleabot.EventContext) error {
		value := strings.TrimSpace(next.Event.Message.PlainText)
		if value != "读者" && value != "作者" {
			_, err := next.Ask(ctx, "请选择角色：读者 / 作者。", options, role)
			return err
		}
		draft.Role = value
		_, err := next.Ask(ctx, "请选择操作：查看 / 编辑。", options, operation)
		return err
	}
	operation = func(ctx context.Context, next *rayleabot.EventContext) error {
		value := strings.TrimSpace(next.Event.Message.PlainText)
		if value != "查看" && value != "编辑" {
			_, err := next.Ask(ctx, "请选择操作：查看 / 编辑。", options, operation)
			return err
		}
		draft.Operation = value
		_, err := next.Ask(ctx, fmt.Sprintf("角色：%s，操作：%s。回复“确认”完成。", draft.Role, draft.Operation), options, confirm)
		return err
	}
	confirm = func(ctx context.Context, next *rayleabot.EventContext) error {
		if strings.TrimSpace(next.Event.Message.PlainText) != "确认" {
			_, err := next.Ask(ctx, "回复“确认”完成本次选择。", options, confirm)
			return err
		}
		if _, err := next.Actions().KVSetWithOptions(ctx, resultKey(next), *draft, rayleabot.KVSetOptions{TTL: 5 * time.Minute}); err != nil {
			return err
		}
		return sendText(ctx, next, fmt.Sprintf("已确认：%s / %s，结果保留五分钟。", draft.Role, draft.Operation))
	}
	_, err := event.Ask(ctx, "请选择角色：读者 / 作者。", options, role)
	return err
}

func sendText(ctx context.Context, event *rayleabot.EventContext, text string) error {
	request := rayleabot.MessageSendRequest{SourceAdapter: event.Event.SourceAdapter, SourceProtocol: event.Event.SourceProtocol, TargetType: event.Event.Target.Type, TargetID: event.Event.Target.ID, Message: rayleabot.MessageOut{Segments: []rayleabot.Segment{rayleabot.Text(text)}}}
	if event.Event.EventType != "session.expired" {
		request.ReplyToEventID = event.Event.EventID
	}
	if _, err := event.Actions().MessageSend(ctx, request); err != nil {
		return err
	}
	return event.Result(nil)
}

func resultKey(event *rayleabot.EventContext) string {
	identity, _ := json.Marshal([6]string{event.Event.SourceProtocol, event.Event.SourceAdapter, event.Bot.ID, event.Event.Target.Type, event.Event.Target.ID, event.Event.Actor.ID})
	return fmt.Sprintf("result:%x", sha256.Sum256(identity))
}
