#!/usr/bin/env python3
"""Example plugin demonstrating governance local actions."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command


class GovernanceControlPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private")

    @command("governance_demo")
    def handle_governance_demo(self, ctx):
        blacklist = ctx.governance_blacklist_read()
        whitelist = ctx.governance_whitelist_read()
        policy = ctx.governance_command_policy_read()

        user_blacklist = blacklist.get("user_entries", [])
        group_blacklist = blacklist.get("group_entries", [])
        user_whitelist = whitelist.get("user_entries", [])
        group_whitelist = whitelist.get("group_entries", [])
        commands = policy.get("commands", [])

        lines = [
            "治理快照：",
            f"- 黑名单用户 {len(user_blacklist)} 条",
            f"- 黑名单群 {len(group_blacklist)} 条",
            f"- 白名单开关 {'开启' if whitelist.get('enabled') else '关闭'}",
            f"- 白名单用户 {len(user_whitelist)} 条",
            f"- 白名单群 {len(group_whitelist)} 条",
            f"- 命令投影 {len(commands)} 条",
        ]
        ctx.send_text("\n".join(lines))

    @command("governance_block")
    def handle_governance_block(self, ctx):
        if not ctx.args:
            ctx.send_text("请提供要加入黑名单的 user_id。")
            return

        target_id = str(ctx.args[0]).strip()
        if not target_id:
            ctx.send_text("user_id 不能为空。")
            return

        ctx.governance_blacklist_write(
            "upsert",
            entry_type="user",
            target_id=target_id,
            reason="example_plugin_demo",
        )
        ctx.send_text(f"已写入黑名单示例条目：{target_id}")


if __name__ == "__main__":
    GovernanceControlPlugin().run()
