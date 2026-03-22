#!/usr/bin/env python3
"""Echo plugin demonstrating command parsing and message segments."""

import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin
from rayleabot import protocol

plugin = RayleaBotPlugin()
plugin.subscribe("message.group", "message.private")


@plugin.on_command("echo", aliases=["repeat"])
def handle_echo(event, request_id):
    target = event.get("target", {})
    payload = event.get("payload", {})
    args = payload.get("args", [])
    text = " ".join(args) if args else event.get("message", {}).get("plain_text", "")

    if not text.strip():
        text = "(空消息)"

    protocol.send_action(plugin._plugin_id, request_id, "message.send", {
        "target_type": target.get("type", "group"),
        "target_id": target.get("id", ""),
        "text": text,
    })


if __name__ == "__main__":
    plugin.run()
