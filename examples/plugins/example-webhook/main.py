#!/usr/bin/env python3
"""Example plugin demonstrating event.expose_webhook."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin
from rayleabot import protocol

plugin = RayleaBotPlugin()
plugin.subscribe("message.group", "webhook.received")


@plugin.on_command("webhook_register")
def handle_register(_event, request_id):
    result = plugin.expose_webhook(
        request_id,
        route="github",
        secret_ref="webhook.github.secret",
        auth_strategy="hmac_sha256",
        header="X-Hub-Signature-256",
        signature_prefix="sha256=",
    )
    protocol.send_result(plugin._plugin_id, request_id, {"webhook": result})


@plugin.on_event("webhook.received")
def handle_webhook(event, request_id):
    payload = event.get("raw_payload", {})
    plugin.logger_write(request_id, "info", "webhook received", {"route": event.get("target", {}).get("id")})
    protocol.send_result(
        plugin._plugin_id,
        request_id,
        {
            "handled": True,
            "raw_payload_keys": sorted(payload.keys()) if isinstance(payload, dict) else [],
        },
    )


if __name__ == "__main__":
    plugin.run()
