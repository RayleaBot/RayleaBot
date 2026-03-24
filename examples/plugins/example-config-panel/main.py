#!/usr/bin/env python3
"""Example plugin demonstrating config.read and config.write."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin
from rayleabot import protocol

plugin = RayleaBotPlugin()
plugin.subscribe("message.group", "message.private")


@plugin.on_command("config_panel")
def handle_config_panel(event, request_id):
    payload = event.get("payload", {})
    args = payload.get("args", [])
    current = plugin.config_read(request_id, ["default_city", "unit"])

    if args:
        plugin.config_write(request_id, {"default_city": args[0]})
        current["values"]["default_city"] = args[0]

    protocol.send_result(
        plugin._plugin_id,
        request_id,
        {
            "handled": True,
            "config": current.get("values", {}),
        },
    )


if __name__ == "__main__":
    plugin.run()
