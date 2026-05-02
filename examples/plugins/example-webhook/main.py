#!/usr/bin/env python3
"""Example plugin demonstrating event.expose_webhook."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command, event_handler


class WebhookPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "webhook.received")

    @command("webhook_register")
    def handle_register(self, ctx):
        result = ctx.expose_webhook(
            route="github",
            secret_ref="webhook.github.secret",
            auth_strategy="hmac_sha256",
            header="X-Hub-Signature-256",
            signature_prefix="sha256=",
        )
        ctx.send_result({"webhook": result})


    @event_handler("webhook.received")
    def handle_webhook(self, ctx):
        payload = ctx.event.get("raw_payload", {})
        ctx.logger_write("info", "webhook received", {"route": ctx.target.get("id")})
        ctx.send_result({
            "handled": True,
            "raw_payload_keys": sorted(payload.keys()) if isinstance(payload, dict) else [],
        })


if __name__ == "__main__":
    WebhookPlugin().run()
