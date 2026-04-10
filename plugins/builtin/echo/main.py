#!/usr/bin/env python3
"""Built-in echo plugin for RayleaBot."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin

plugin = RayleaBotPlugin()
plugin.subscribe("message.group", "message.private")


@plugin.on_command("echo")
def handle_echo(event, request_id):
    target = event.get("target", {})
    payload = event.get("payload", {})
    args = payload.get("args", [])
    text = " ".join(args)

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
