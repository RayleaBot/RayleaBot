#!/usr/bin/env python3
"""Notice logger plugin demonstrating notice event handling."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin
from rayleabot import protocol

plugin = RayleaBotPlugin()
plugin.subscribe("notice.member_increase", "notice.member_decrease")


@plugin.on_event("notice.member_increase")
def handle_join(event, request_id):
    actor = event.get("actor", {})
    target = event.get("target", {})
    payload = event.get("payload", {})
    plugin.logger_write(
        request_id,
        "info",
        "member joined notice received",
        {
            "user_id": actor.get("id"),
            "group_id": target.get("id"),
            "sub_type": payload.get("sub_type"),
        },
    )
    counter = plugin.storage_get(request_id, "notice:member_increase:count")
    current = 0
    if counter.get("exists"):
        current = int(counter.get("value", 0))
    plugin.storage_set(request_id, "notice:member_increase:count", current + 1)
    protocol.send_result(plugin._plugin_id, request_id, {"logged": True})


@plugin.on_event("notice.member_decrease")
def handle_leave(event, request_id):
    actor = event.get("actor", {})
    target = event.get("target", {})
    payload = event.get("payload", {})
    plugin.logger_write(
        request_id,
        "info",
        "member left notice received",
        {
            "user_id": actor.get("id"),
            "group_id": target.get("id"),
            "sub_type": payload.get("sub_type"),
        },
    )
    counter = plugin.storage_get(request_id, "notice:member_decrease:count")
    current = 0
    if counter.get("exists"):
        current = int(counter.get("value", 0))
    plugin.storage_set(request_id, "notice:member_decrease:count", current + 1)
    protocol.send_result(plugin._plugin_id, request_id, {"logged": True})


if __name__ == "__main__":
    plugin.run()
