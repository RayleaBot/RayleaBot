#!/usr/bin/env python3
"""Built-in help plugin for RayleaBot."""

import sys
import os

# Add SDK to path for local development.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin
from rayleabot import protocol

plugin = RayleaBotPlugin()
plugin.subscribe("message.group", "message.private")


@plugin.on_command("help", aliases=["commands"])
def handle_help(event, request_id):
    target = event.get("target", {})
    text = "可用命令:\n/help - 显示所有可用命令\n\n使用 /help <命令名> 查看详细说明。"
    protocol.send_action(plugin._plugin_id, request_id, "message.send", {
        "target_type": target.get("type", "group"),
        "target_id": target.get("id", ""),
        "text": text,
    })


if __name__ == "__main__":
    plugin.run()
