#!/usr/bin/env python3
"""Example plugin showing scoped http.request and storage.file usage."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin
from rayleabot import protocol

plugin = RayleaBotPlugin()
plugin.subscribe("message.group", "message.private")


@plugin.on_command("scope_fetch", aliases=["scope_cache"])
def handle_scope_fetch(event, request_id):
    response = plugin.http_request(request_id, "GET", "https://example.com/")
    cached_path = "cache/example.html"

    if "body_text" in response:
        plugin.storage_file_write(request_id, cached_path, content_text=response["body_text"])
    else:
        cached_path = "cache/example.bin"
        plugin.storage_file_write(request_id, cached_path, content_base64=response["body_base64"])

    plugin.logger_write(
        request_id,
        "info",
        "scoped content cached",
        {
            "status_code": response.get("status_code"),
            "cached_path": cached_path,
        },
    )
    protocol.send_result(
        plugin._plugin_id,
        request_id,
        {
            "handled": True,
            "cached_path": cached_path,
        },
    )


if __name__ == "__main__":
    plugin.run()
