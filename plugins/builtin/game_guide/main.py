#!/usr/bin/env python3
"""Built-in game guide plugin for RayleaBot."""

import os
import sys

PLUGIN_DIR = os.path.dirname(__file__)
sys.path.insert(0, PLUGIN_DIR)
sys.path.insert(0, os.path.join(PLUGIN_DIR, "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, event_handler

from raylea_game_guide import GameGuideService


class GameGuidePlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("plugin.started", "bot.identity.changed")
        self._service = GameGuideService(plugin_dir=PLUGIN_DIR)

    @event_handler()
    def handle_message(self, ctx):
        try:
            self._service.handle_message(ctx)
        except Exception as exc:
            try:
                ctx.logger_write("warn", "游戏攻略查询失败", {"error": str(exc)})
            except Exception:
                pass
            ctx.send_text("攻略图查询失败，请稍后再试。")


if __name__ == "__main__":
    GameGuidePlugin().run()
