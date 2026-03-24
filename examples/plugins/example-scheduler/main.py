#!/usr/bin/env python3
"""Example plugin demonstrating scheduler.create."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin
from rayleabot import protocol

plugin = RayleaBotPlugin()
plugin.subscribe("message.group", "scheduler.trigger")


@plugin.on_command("schedule_daily")
def handle_schedule(_event, request_id):
    result = plugin.scheduler_create(
        request_id,
        "daily_morning_report",
        "0 8 * * *",
        {"report_type": "daily"},
    )
    protocol.send_result(plugin._plugin_id, request_id, {"scheduled": result})


@plugin.on_event("scheduler.trigger")
def handle_trigger(event, request_id):
    protocol.send_result(
        plugin._plugin_id,
        request_id,
        {
            "handled": True,
            "payload": event.get("payload", {}),
        },
    )


if __name__ == "__main__":
    plugin.run()
