#!/usr/bin/env python3
"""Example plugin demonstrating plugin.list."""

from rayleabot_runtime import RayleaBotPlugin, command


class PluginListPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private")

    @command("plugins_demo")
    def handle_plugins_demo(self, ctx):
        result = ctx.plugin_list()
        items = result.get("items", [])
        names = [item.get("name") or item.get("id") or "" for item in items]
        text = "已加载插件：\n" + "\n".join(name for name in names if name)

        ctx.send_text(text or "当前没有插件。")


if __name__ == "__main__":
    PluginListPlugin().run()
