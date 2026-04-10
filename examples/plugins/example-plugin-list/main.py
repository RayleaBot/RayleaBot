#!/usr/bin/env python3
"""Example plugin demonstrating plugin.list."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin

plugin = RayleaBotPlugin()
plugin.subscribe("message.group", "message.private")


@plugin.on_command("plugins_demo")
def handle_plugins_demo(event, request_id):
    target = event.get("target", {})
    result = plugin.plugin_list(request_id)
    items = result.get("items", [])
    names = [item.get("name") or item.get("id") or "" for item in items]
    text = "已加载插件：\n" + "\n".join(name for name in names if name)

    plugin.send_message(
        request_id,
        target.get("type", "group"),
        target.get("id", ""),
        [{
            "type": "text",
            "data": {"text": text or "当前没有插件。"},
        }],
    )


if __name__ == "__main__":
    plugin.run()
