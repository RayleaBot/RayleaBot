#!/usr/bin/env python3
"""Echo plugin demonstrating command parsing and message segments."""

import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin

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

    plugin.send_message(
        request_id,
        target.get("type", "group"),
        target.get("id", ""),
        [{
            "type": "text",
            "data": {"text": text},
        }],
    )


if __name__ == "__main__":
    plugin.run()
