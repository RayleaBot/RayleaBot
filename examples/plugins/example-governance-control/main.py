#!/usr/bin/env python3
"""Example plugin demonstrating governance local actions."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin

plugin = RayleaBotPlugin()
plugin.subscribe("message.group", "message.private")


def send_text(event, request_id, text):
    target = event.get("target", {})
    plugin.send_message(
        request_id,
        target.get("type", "group"),
        target.get("id", ""),
        [{
            "type": "text",
            "data": {"text": text},
        }],
    )


@plugin.on_command("governance_demo")
def handle_governance_demo(event, request_id):
    blacklist = plugin.governance_blacklist_read(request_id)
    whitelist = plugin.governance_whitelist_read(request_id)
    policy = plugin.governance_command_policy_read(request_id)

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
    send_text(event, request_id, "\n".join(lines))


@plugin.on_command("governance_block")
def handle_governance_block(event, request_id):
    args = (event.get("payload") or {}).get("args") or []
    if not args:
        send_text(event, request_id, "请提供要加入黑名单的 user_id。")
        return

    target_id = str(args[0]).strip()
    if not target_id:
        send_text(event, request_id, "user_id 不能为空。")
        return

    plugin.governance_blacklist_write(
        request_id,
        "upsert",
        entry_type="user",
        target_id=target_id,
        reason="example_plugin_demo",
    )
    send_text(event, request_id, f"已写入黑名单示例条目：{target_id}")


if __name__ == "__main__":
    plugin.run()
