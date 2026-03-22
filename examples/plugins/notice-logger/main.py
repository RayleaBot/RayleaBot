#!/usr/bin/env python3
"""Notice logger plugin demonstrating notice event handling."""

import sys
import os

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
    sys.stderr.write(
        f"[notice-logger] member joined: user={actor.get('id')} "
        f"group={target.get('id')} sub_type={payload.get('sub_type')}\n"
    )
    protocol.send_result(plugin._plugin_id, request_id, {"logged": True})


@plugin.on_event("notice.member_decrease")
def handle_leave(event, request_id):
    actor = event.get("actor", {})
    target = event.get("target", {})
    payload = event.get("payload", {})
    sys.stderr.write(
        f"[notice-logger] member left: user={actor.get('id')} "
        f"group={target.get('id')} sub_type={payload.get('sub_type')}\n"
    )
    protocol.send_result(plugin._plugin_id, request_id, {"logged": True})


if __name__ == "__main__":
    plugin.run()
