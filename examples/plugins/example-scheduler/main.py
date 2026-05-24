#!/usr/bin/env python3
"""Example plugin demonstrating scheduler.create."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command, event_handler


class SchedulerPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "scheduler.trigger")

    @command("schedule_daily")
    def handle_schedule(self, ctx):
        result = ctx.scheduler_create(
            "daily_morning_report",
            "0 8 * * *",
            {"report_type": "daily"},
            log_label="每日早报",
        )
        ctx.send_result({"scheduled": result})

    @event_handler("scheduler.trigger")
    def handle_trigger(self, ctx):
        ctx.send_result({
            "handled": True,
            "payload": ctx.payload,
        })


if __name__ == "__main__":
    SchedulerPlugin().run()
