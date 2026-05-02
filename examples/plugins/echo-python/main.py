#!/usr/bin/env python3
"""Echo plugin demonstrating command parsing and message segments."""

import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command


class EchoPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private")

    @command("echo", aliases=["repeat"])
    def handle_echo(self, ctx):
        text = " ".join(ctx.args) if ctx.args else ctx.plain_text

        if not text.strip():
            text = "(空消息)"

        ctx.send_text(text)


if __name__ == "__main__":
    EchoPlugin().run()
