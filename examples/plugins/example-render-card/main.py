#!/usr/bin/env python3
"""Example plugin demonstrating render.image."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command


class RenderCardPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private")

    @command("render_card")
    def handle_render(self, ctx):
        result = ctx.render_image(
            "help.menu",
            {
                "title": "Render Example",
                "items": [
                    {"name": "weather", "description": "Query weather"},
                    {"name": "echo", "description": "Repeat text"},
                ],
            },
            theme="default",
            output="png",
            fallback_text="Render unavailable.",
        )

        ctx.send_message([
            {
                "type": "image",
                "data": {
                    "file": result.get("image_path", ""),
                },
            }
        ])


if __name__ == "__main__":
    RenderCardPlugin().run()
