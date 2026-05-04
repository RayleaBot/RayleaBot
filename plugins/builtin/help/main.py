#!/usr/bin/env python3
"""Built-in help plugin for RayleaBot."""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command


def normalize_usage(usage, prefix, command_tokens):
    usage = (usage or "").strip()
    if not usage:
        return ""

    parts = usage.split(None, 1)
    head = parts[0]
    tail = parts[1] if len(parts) > 1 else ""

    cleaned = head
    while cleaned and not (cleaned[0].isalnum() or cleaned[0] in "._-"):
        cleaned = cleaned[1:]

    tokens = {token.casefold() for token in command_tokens if token}
    if cleaned.casefold() in tokens:
        return f"{prefix}{cleaned} {tail}".strip()

    return usage


def usage_query_tokens(usage):
    usage = (usage or "").strip()
    if not usage:
        return []

    head = usage.split(None, 1)[0]
    cleaned = head
    while cleaned and not (cleaned[0].isalnum() or cleaned[0] in "._-"):
        cleaned = cleaned[1:]
    return [cleaned] if cleaned else []


def normalize_command(command, prefix):
    name = (command.get("name") or "").strip()
    aliases = [alias.strip() for alias in command.get("aliases") or [] if alias and alias.strip()]
    declaration_id = (command.get("declaration_id") or "").strip()
    tokens = [name] + aliases
    usage = normalize_usage(command.get("usage"), prefix, tokens)
    query_tokens = usage_query_tokens(usage)
    if declaration_id:
        query_tokens.append(declaration_id)
    return {
        "name": name,
        "aliases": aliases,
        "description": (command.get("description") or "").strip(),
        "usage": usage,
        "query_tokens": query_tokens,
        "permission": (command.get("permission") or "").strip(),
        "command_source": (command.get("command_source") or "").strip(),
        "declaration_id": declaration_id,
    }


def normalize_plugin_item(item, prefix):
    commands = [normalize_command(command, prefix) for command in item.get("commands") or [] if (command.get("name") or "").strip()]
    conflicts = {(conflict or "").strip().casefold() for conflict in item.get("command_conflicts") or [] if (conflict or "").strip()}
    return {
        "id": (item.get("id") or "").strip(),
        "name": (item.get("name") or item.get("id") or "").strip(),
        "description": (item.get("description") or "").strip(),
        "registration_state": (item.get("registration_state") or "").strip(),
        "desired_state": (item.get("desired_state") or "").strip(),
        "commands": commands,
        "command_conflicts": conflicts,
        "query_key": select_query_key(item, commands, conflicts),
    }


def select_query_key(item, commands, conflicts):
    for command in commands:
        name = command["name"]
        if name and name.casefold() not in conflicts:
            return name
    return (item.get("id") or "").strip()


def format_command_label(command, prefix):
    label = f"{prefix}{command['name']}"
    if command["aliases"]:
        alias_text = "、".join(f"{prefix}{alias}" for alias in command["aliases"])
        return f"{label} · 别名 {alias_text}"
    return label


def format_command_description(command):
    parts = []
    if command["description"]:
        parts.append(command["description"])
    if command["permission"]:
        parts.append(f"权限：{command['permission']}")
    if parts:
        return " | ".join(parts)
    return "未提供指令说明"


def build_root_render_data(items, prefix):
    return {
        "title": "帮助菜单",
        "subtitle": f"使用 {prefix}help <目标> 查看插件说明",
        "items": [
            {
                "name": item["name"],
                "description": item["description"] or "未提供插件说明",
                "usage": f"{prefix}help {item['query_key']}",
            }
            for item in items
        ],
    }


def build_root_text(items, prefix):
    lines = ["帮助菜单"]
    if items:
        for item in items:
            lines.append(f"{item['name']} - {item['description'] or '未提供插件说明'}")
            lines.append(f"查看方式：{prefix}help {item['query_key']}")
    else:
        lines.append("当前没有可展示的插件指令。")
    lines.append("")
    lines.append(f"使用 {prefix}help <目标> 查看插件说明。")
    return "\n".join(lines)


def build_plugin_render_data(item, prefix):
    subtitle_parts = [f"插件 ID：{item['id']}"]
    if item["description"]:
        subtitle_parts.append(item["description"])
    return {
        "title": item["name"],
        "subtitle": " | ".join(subtitle_parts),
        "items": [
            {
                "name": format_command_label(command, prefix),
                "description": format_command_description(command),
                "usage": command["usage"],
            }
            for command in item["commands"]
        ],
    }


def build_plugin_text(item, prefix):
    lines = [item["name"]]
    lines.append(f"插件 ID：{item['id']}")
    if item["description"]:
        lines.append(item["description"])
    lines.append("")
    if not item["commands"]:
        lines.append("这个插件没有声明可用指令。")
        return "\n".join(lines)

    lines.append("可用指令：")
    for command in item["commands"]:
        lines.append(format_command_label(command, prefix))
        lines.append(format_command_description(command))
        if command["usage"]:
            lines.append(f"用法：{command['usage']}")
        lines.append("")
    return "\n".join(line for line in lines).strip()


def find_plugin(items, query):
    exact_id = [item for item in items if item["id"] == query]
    if exact_id:
        return "plugin", exact_id[0]

    lowered = query.casefold()
    exact_name = [item for item in items if item["name"].casefold() == lowered]
    if exact_name:
        return "plugin", exact_name[0]

    command_matches = []
    for item in items:
        for command in item["commands"]:
            tokens = [command["name"]] + command["aliases"] + command["query_tokens"]
            if any(token.casefold() == lowered for token in tokens if token):
                command_matches.append(item)
                break

    if len(command_matches) == 1:
        return "plugin", command_matches[0]
    if len(command_matches) > 1:
        unique_ids = []
        seen = set()
        for item in command_matches:
            if item["id"] in seen:
                continue
            seen.add(item["id"])
            unique_ids.append(item["id"])
        return "ambiguous", unique_ids

    return "missing", None


class HelpPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private")

    def visible_plugins(self, request_id):
        prefix = self.primary_command_prefix or "/"
        response = self.plugin_list(request_id)
        items = []
        for item in response.get("items", []):
            normalized = normalize_plugin_item(item, prefix)
            if normalized["registration_state"] != "installed":
                continue
            if normalized["desired_state"] != "enabled":
                continue
            if not normalized["commands"]:
                continue
            items.append(normalized)
        return items

    def try_render_image(self, ctx, render_data, fallback_text):
        try:
            result = ctx.render_image(
                "help.menu",
                render_data,
                theme="default",
                output="png",
                fallback_text=fallback_text,
            )
        except Exception:
            return False

        image_path = (result.get("image_path") or "").strip()
        if not image_path:
            return False

        ctx.send_message([{
            "type": "image",
            "data": {"file": image_path},
        }])
        return True

    @command("help", aliases=["commands"])
    def handle_help(self, ctx):
        prefix = self.primary_command_prefix or "/"
        query = " ".join(part.strip() for part in ctx.args if isinstance(part, str) and part.strip()).strip()

        try:
            items = self.visible_plugins(ctx.request_id)
        except Exception:
            ctx.send_text("帮助暂时不可用。")
            return

        if not query:
            fallback_text = build_root_text(items, prefix)
            if self.try_render_image(ctx, build_root_render_data(items, prefix), fallback_text):
                return
            ctx.send_text(fallback_text)
            return

        match_type, match_value = find_plugin(items, query)
        if match_type == "plugin":
            fallback_text = build_plugin_text(match_value, prefix)
            if self.try_render_image(ctx, build_plugin_render_data(match_value, prefix), fallback_text):
                return
            ctx.send_text(fallback_text)
            return

        if match_type == "ambiguous":
            text = [
                f"“{query}” 对应多个插件。",
                "可用插件 ID：",
            ]
            text.extend(match_value)
            text.append("")
            text.append(f"使用 {prefix}help <plugin.id> 查看具体说明。")
            ctx.send_text("\n".join(text))
            return

        ctx.send_text(f"没有找到与“{query}”对应的插件或指令。\n使用 {prefix}help 查看插件菜单。")


if __name__ == "__main__":
    HelpPlugin().run()
