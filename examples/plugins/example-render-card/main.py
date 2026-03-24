#!/usr/bin/env python3
"""Example plugin demonstrating render.image."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin

plugin = RayleaBotPlugin()
plugin.subscribe("message.group", "message.private")


@plugin.on_command("render_card")
def handle_render(event, request_id):
    target = event.get("target", {})
    result = plugin.render_image(
        request_id,
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

    plugin.send_message(
        request_id,
        target.get("type", "group"),
        target.get("id", ""),
        [
            {
                "type": "image",
                "data": {
                    "file": result.get("image_path", ""),
                },
            }
        ],
    )


if __name__ == "__main__":
    plugin.run()
