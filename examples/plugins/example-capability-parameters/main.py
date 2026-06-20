#!/usr/bin/env python3
"""Example plugin showing http.request and storage.file capability parameters."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command


class CapabilityParametersPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private")

    @command("scope_fetch", aliases=["scope_cache"])
    def handle_scope_fetch(self, ctx):
        response = ctx.http_request("GET", "https://example.com/")
        cached_path = "cache/example.html"

        if "body_text" in response:
            ctx.storage_file_write(cached_path, content_text=response["body_text"])
        else:
            cached_path = "cache/example.bin"
            ctx.storage_file_write(cached_path, content_base64=response["body_base64"])

        ctx.logger_write(
            "info",
            "scoped content cached",
            {
                "status_code": response.get("status_code"),
                "cached_path": cached_path,
            },
        )
        ctx.send_result({
            "handled": True,
            "cached_path": cached_path,
        })


if __name__ == "__main__":
    CapabilityParametersPlugin().run()
