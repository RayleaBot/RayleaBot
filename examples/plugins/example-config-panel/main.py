#!/usr/bin/env python3
"""Example plugin demonstrating config.read and config.write."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command


class ConfigPanelPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private")

    @command("config_panel")
    def handle_config_panel(self, ctx):
        current = ctx.config_read(["default_city", "unit"])

        if ctx.args:
            ctx.config_write({"default_city": ctx.args[0]})
            current["values"]["default_city"] = ctx.args[0]

        ctx.send_result({
            "handled": True,
            "config": current.get("values", {}),
        })


if __name__ == "__main__":
    ConfigPanelPlugin().run()
