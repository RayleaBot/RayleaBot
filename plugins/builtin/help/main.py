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


def normalize_help_item(item, prefix):
    title = (item.get("title") or "").strip()
    command = (item.get("command") or "").strip()
    usage = normalize_usage(item.get("usage"), prefix, [command] if command else [])
    return {
        "title": title,
        "name": title,
        "description": (item.get("description") or "").strip(),
        "usage": usage,
        "command": command,
        "permission": (item.get("permission") or "").strip(),
        "permission_label": format_permission_label((item.get("permission") or "").strip()),
    }


def normalize_help(help, prefix):
    if not isinstance(help, dict):
        return None
    groups = []
    for group in help.get("groups") or []:
        if not isinstance(group, dict):
            continue
        title = (group.get("title") or "").strip()
        items = [normalize_help_item(item, prefix) for item in group.get("items") or [] if isinstance(item, dict) and (item.get("title") or "").strip()]
        if title and items:
            groups.append({"title": title, "items": items})
    if not groups:
        return None
    return {
        "title": (help.get("title") or "").strip(),
        "summary": (help.get("summary") or "").strip(),
        "groups": groups,
    }


def normalize_plugin_item(item, prefix):
    commands = [normalize_command(command, prefix) for command in item.get("commands") or [] if (command.get("name") or "").strip()]
    help_data = normalize_help(item.get("help"), prefix)
    conflicts = {(conflict or "").strip().casefold() for conflict in item.get("command_conflicts") or [] if (conflict or "").strip()}
    plugin_id = (item.get("id") or "").strip()
    name = (item.get("name") or plugin_id).strip()
    return {
        "id": plugin_id,
        "name": name,
        "description": (item.get("description") or "").strip(),
        "registration_state": (item.get("registration_state") or "").strip(),
        "desired_state": (item.get("desired_state") or "").strip(),
        "commands": commands,
        "help": help_data,
        "command_conflicts": conflicts,
        "query_key": select_plugin_query_key(plugin_id, name),
    }


def select_plugin_query_key(plugin_id, name):
    return name or plugin_id


def format_command_label(command, prefix):
    label = f"{prefix}{command['name']}"
    if command["aliases"]:
        alias_text = "、".join(f"{prefix}{alias}" for alias in command["aliases"])
        return f"{label} · 别名 {alias_text}"
    return label


def format_command_description(command):
    if command["description"]:
        return command["description"]
    return "未提供指令说明"


def format_permission_label(permission):
    labels = {
        "everyone": "所有人可用",
        "group_admin": "管理员可用",
        "super_admin": "超级管理员可用",
    }
    return labels.get(permission, "")


def format_permission_text(permission):
    label = format_permission_label(permission)
    return f"开放范围：{label}" if label else ""


def command_render_item(command, prefix):
    item = {
        "name": format_command_label(command, prefix),
        "description": format_command_description(command),
        "usage": command["usage"],
    }
    permission = command["permission"]
    permission_label = format_permission_label(permission)
    if permission:
        item["permission"] = permission
    if permission_label:
        item["permission_label"] = permission_label
    return item


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


def has_visible_help(item):
    help_data = item.get("help")
    return bool(help_data and help_data.get("groups"))


def help_groups_as_render_groups(help_data):
    if not help_data:
        return []
    groups = []
    for group in help_data.get("groups") or []:
        items = []
        for entry in group.get("items") or []:
            item = {
                "name": entry["name"],
                "title": entry["title"],
                "description": entry["description"] or entry["title"],
                "usage": entry["usage"],
            }
            if entry["permission"]:
                item["permission"] = entry["permission"]
            if entry["permission_label"]:
                item["permission_label"] = entry["permission_label"]
            items.append(item)
        if items:
            groups.append({"title": group["title"], "items": items})
    return groups


def compact_help_groups_as_text(help_data, limit=8):
    lines = []
    count = 0
    for group in (help_data or {}).get("groups") or []:
        visible_items = [item for item in group.get("items") or [] if item.get("title")]
        if not visible_items:
            continue
        lines.append(group["title"])
        for item in visible_items:
            if count >= limit:
                lines.append("更多内容请查看帮助图片。")
                return lines
            usage = item.get("usage") or item.get("title")
            if usage and usage != item.get("title"):
                lines.append(f"- {item['title']}：{usage}")
            else:
                lines.append(f"- {item['title']}")
            count += 1
    return lines


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
        "groups": help_groups_as_render_groups(item["help"]),
        "items": [command_render_item(command, prefix) for command in item["commands"]],
    }


def build_plugin_text(item, prefix):
    lines = [item["name"]]
    lines.append(f"插件 ID：{item['id']}")
    if item["description"]:
        lines.append(item["description"])
    lines.append("")
    if has_visible_help(item):
        lines.extend(compact_help_groups_as_text(item["help"]))
        return "\n".join(line for line in lines).strip()

    if not item["commands"]:
        lines.append("这个插件没有声明可用指令。")
        return "\n".join(lines)

    lines.append("可用指令：")
    for command in item["commands"]:
        lines.append(format_command_label(command, prefix))
        lines.append(format_command_description(command))
        permission_text = format_permission_text(command["permission"])
        if permission_text:
            lines.append(permission_text)
        if command["usage"]:
            lines.append(f"用法：{command['usage']}")
        lines.append("")
    return "\n".join(line for line in lines).strip()


def build_command_render_data(item, command, prefix):
    subtitle_parts = [item["name"]]
    if item["id"]:
        subtitle_parts.append(f"插件 ID：{item['id']}")
    return {
        "title": command["name"],
        "subtitle": " | ".join(subtitle_parts),
        "items": [command_render_item(command, prefix)],
    }


def build_command_text(item, command, prefix):
    lines = [command["name"]]
    lines.append(f"插件：{item['name']}")
    if item["id"]:
        lines.append(f"插件 ID：{item['id']}")
    lines.append("")
    lines.append(format_command_label(command, prefix))
    lines.append(format_command_description(command))
    permission_text = format_permission_text(command["permission"])
    if permission_text:
        lines.append(permission_text)
    if command["usage"]:
        lines.append(f"用法：{command['usage']}")
    return "\n".join(lines).strip()


def find_help_target(items, query):
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
                command_matches.append((item, command))
        for group in (item.get("help") or {}).get("groups") or []:
            for help_item in group.get("items") or []:
                tokens = [help_item.get("title"), help_item.get("command")]
                if any(str(token or "").casefold() == lowered for token in tokens if token):
                    command = next((cmd for cmd in item["commands"] if cmd["name"] == help_item.get("command")), None)
                    if command:
                        command_matches.append((item, command))

    if len(command_matches) == 1:
        return "command", command_matches[0]
    if len(command_matches) > 1:
        unique_targets = []
        seen = set()
        for item, command in command_matches:
            target = f"{item['id']}:{command['name']}"
            if target in seen:
                continue
            seen.add(target)
            unique_targets.append(f"{item['id']} / {command['name']}")
        return "ambiguous", unique_targets

    return "missing", None


class HelpPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private")

    def visible_plugins(self, request_id):
        prefix = self.primary_command_prefix or "/"
        response = self.plugin_list(request_id, visibility="caller")
        items = []
        for item in response.get("items", []):
            normalized = normalize_plugin_item(item, prefix)
            if normalized["registration_state"] != "installed":
                continue
            if normalized["desired_state"] != "enabled":
                continue
            if not normalized["commands"] and not has_visible_help(normalized):
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
        except Exception as exc:
            self.log_render_failure(ctx, exc)
            return False

        image_path = (result.get("image_path") or "").strip()
        if not image_path:
            self.log_render_failure(ctx, "missing image_path")
            return False

        ctx.send_message([{
            "type": "image",
            "data": {"file": image_path},
        }])
        return True

    def log_render_failure(self, ctx, error):
        try:
            ctx.logger_write("warn", "帮助菜单图片渲染失败", {"error": str(error)})
        except Exception:
            pass

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

        match_type, match_value = find_help_target(items, query)
        if match_type == "plugin":
            fallback_text = build_plugin_text(match_value, prefix)
            if self.try_render_image(ctx, build_plugin_render_data(match_value, prefix), fallback_text):
                return
            ctx.send_text(fallback_text)
            return

        if match_type == "command":
            item, command = match_value
            fallback_text = build_command_text(item, command, prefix)
            if self.try_render_image(ctx, build_command_render_data(item, command, prefix), fallback_text):
                return
            ctx.send_text(fallback_text)
            return

        if match_type == "ambiguous":
            text = [
                f"“{query}” 对应多个指令。",
                "可用目标：",
            ]
            text.extend(match_value)
            text.append("")
            text.append(f"使用 {prefix}help <插件名或指令名> 查看具体说明。")
            ctx.send_text("\n".join(text))
            return

        ctx.send_text(f"没有找到与“{query}”对应的插件或指令。\n使用 {prefix}help 查看插件菜单。")


if __name__ == "__main__":
    HelpPlugin().run()
