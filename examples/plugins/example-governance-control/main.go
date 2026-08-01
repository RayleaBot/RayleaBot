package main

import (
	"context"
	"fmt"
	"os"

	rayleabot "github.com/RayleaBot/RayleaBot/sdk/go"
)

func main() {
	err := rayleabot.Run(context.Background(), rayleabot.Options{PluginID: "example-governance-control", Subscriptions: []string{"message.group", "message.private"}}, rayleabot.HandlerFunc(handle))
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func handle(ctx context.Context, event *rayleabot.EventContext) error {
	switch event.Event.Command() {
	case "governance_demo":
		blacklist, err := event.Actions().GovernanceBlacklistRead(ctx)
		if err != nil {
			return err
		}
		whitelist, err := event.Actions().GovernanceWhitelistRead(ctx)
		if err != nil {
			return err
		}
		policy, err := event.Actions().GovernanceCommandPolicyRead(ctx)
		if err != nil {
			return err
		}
		return event.SendText(fmt.Sprintf("治理快照\n黑名单：%d 个字段\n白名单：%d 个字段\n命令策略：%d 个字段", len(blacklist), len(whitelist), len(policy)))
	case "governance_block":
		args := event.Event.Args()
		if len(args) == 0 {
			return event.SendText("请提供 user_id。")
		}
		_, err := event.Actions().GovernanceBlacklistWrite(ctx, rayleabot.GovernanceBlacklistWriteRequest{Operation: "upsert", EntryType: "user", TargetID: args[0], Reason: "example_plugin_demo"})
		if err != nil {
			return err
		}
		return event.SendText("已写入黑名单示例条目：" + args[0])
	default:
		return event.Result(map[string]any{"handled": false})
	}
}
