#!/usr/bin/env python3
"""Minimal class-based Python plugin example."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, event_handler


class HelloPythonPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group")

    @event_handler("message.group")
    def handle_group_message(self, ctx):
        ctx.send_result({
            "handled": True,
            "summary": f"hello-python accepted {ctx.event_type}",
        })


if __name__ == "__main__":
    HelloPythonPlugin().run()
