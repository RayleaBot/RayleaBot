#!/usr/bin/env python3
"""Notice logger plugin demonstrating notice event handling."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, event_handler


class NoticeLoggerPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("notice.member_increase", "notice.member_decrease")

    def log_notice(self, ctx, event_name, counter_key):
        ctx.logger_write(
            "info",
            event_name,
            {
                "user_id": ctx.actor.get("id"),
                "group_id": ctx.target.get("id"),
                "sub_type": ctx.payload.get("sub_type"),
            },
        )
        counter = ctx.storage_get(counter_key)
        current = 0
        if counter.get("exists"):
            current = int(counter.get("value", 0))
        ctx.storage_set(counter_key, current + 1)
        ctx.send_result({"logged": True})

    @event_handler("notice.member_increase")
    def handle_join(self, ctx):
        self.log_notice(ctx, "member joined notice received", "notice:member_increase:count")

    @event_handler("notice.member_decrease")
    def handle_leave(self, ctx):
        self.log_notice(ctx, "member left notice received", "notice:member_decrease:count")


if __name__ == "__main__":
    NoticeLoggerPlugin().run()
