#!/usr/bin/env python3
"""Built-in subscription hub plugin for RayleaBot."""

import copy
import os
import random
import sys

PLUGIN_DIR = os.path.dirname(__file__)
sys.path.insert(0, PLUGIN_DIR)
sys.path.insert(0, os.path.join(PLUGIN_DIR, "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command, event_handler

from bilibili import (
    DYNAMIC_URL,
    LIVE_URL,
    build_cookie_headers,
    dynamic_updates,
    live_update,
    parse_json_response,
)
from rendering import build_fallback_text, build_render_data
from settings import SETTINGS_KEYS, merge_settings, normalize_settings


DEFAULT_SETTINGS_PATH = os.path.join(PLUGIN_DIR, "default_config.json")


def load_default_settings(path=DEFAULT_SETTINGS_PATH):
    import json
    with open(path, "r", encoding="utf-8") as handle:
        return normalize_settings(json.load(handle))


def storage_value(result, fallback=None):
    if isinstance(result, dict):
        if result.get("exists") and "value" in result:
            return result.get("value")
        if "value" in result:
            return result.get("value")
    return fallback


class SubscriptionHubPlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private", "scheduler.trigger", "config.changed")
        self._default_settings = load_default_settings()
        self._settings = copy.deepcopy(self._default_settings)
        self._settings_loaded = False

    def load_settings(self, ctx, force=False):
        if self._settings_loaded and not force:
            return copy.deepcopy(self._settings)
        try:
            response = ctx.config_read(SETTINGS_KEYS)
            values = response.get("values", {}) if isinstance(response, dict) else {}
            self._settings = merge_settings(self._default_settings, values)
            self._settings_loaded = True
        except Exception as exc:
            self._settings = copy.deepcopy(self._default_settings)
            self.try_log(ctx, "warn", "订阅设置读取失败，使用默认设置", {"error": str(exc)})
        return copy.deepcopy(self._settings)

    def try_log(self, ctx, level, message, fields=None):
        try:
            ctx.logger_write(level, message, fields or {})
        except Exception:
            pass

    @command("订阅状态")
    def handle_status_command(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(build_status_text(settings))
        ctx.send_result({"handled": True})

    @event_handler("config.changed")
    def handle_config_changed(self, ctx):
        settings = self.load_settings(ctx, force=True)
        self.ensure_scheduler(ctx, settings)
        ctx.send_result({"handled": True})

    @event_handler("scheduler.trigger")
    def handle_scheduler_trigger(self, ctx):
        settings = self.load_settings(ctx, force=True)
        self.ensure_scheduler(ctx, settings)
        if not settings["enabled"]:
            ctx.send_result({"handled": True, "skipped": "disabled"})
            return
        sent = self.poll_all(ctx, settings)
        ctx.send_result({"handled": True, "sent": sent})

    def ensure_scheduler(self, ctx, settings):
        try:
            ctx.scheduler_create(
                "subscription-hub-poll",
                settings["poll_cron"],
                payload={"kind": "subscription_poll"},
            )
        except Exception as exc:
            self.try_log(ctx, "warn", "订阅轮询任务注册失败", {"error": str(exc)})

    def poll_all(self, ctx, settings):
        sent = 0
        for subscription in settings["subscriptions"]:
            if sent >= settings["max_updates_per_poll"]:
                break
            if not subscription.get("enabled", True):
                continue
            if subscription.get("platform") != "bilibili":
                continue
            sent += self.poll_bilibili_subscription(ctx, settings, subscription, settings["max_updates_per_poll"] - sent)
        return sent

    def poll_bilibili_subscription(self, ctx, settings, subscription, remaining):
        token = self.choose_token(ctx, settings)
        headers = build_cookie_headers(token)
        updates = []
        services = set(subscription.get("services") or ["all"])
        if "all" in services or any(service in services for service in {"video", "image_text", "article", "repost"}):
            response = ctx.http_request(
                "GET",
                DYNAMIC_URL.format(uid=subscription["uid"]),
                headers=headers,
                timeout_seconds=settings["poll_timeout_seconds"],
            )
            updates.extend(dynamic_updates(parse_json_response(response)))
        if "all" in services or "live" in services:
            response = ctx.http_request(
                "GET",
                LIVE_URL.format(uid=subscription["uid"]),
                headers=headers,
                timeout_seconds=settings["poll_timeout_seconds"],
            )
            update = live_update(parse_json_response(response), subscription["uid"])
            if update and (update.get("live_status") == 1 or self.previous_live_status(ctx, subscription) == 1):
                updates.append(update)

        sent = 0
        for update in updates:
            if sent >= remaining:
                break
            if not service_enabled(subscription, update.get("service")):
                continue
            if self.seen_update(ctx, subscription, update):
                continue
            self.push_update(ctx, subscription, update)
            self.mark_seen(ctx, subscription, update)
            sent += 1
        return sent

    def choose_token(self, ctx, settings):
        candidates = [item for item in settings["tokens"] if item.get("enabled", True)]
        random.shuffle(candidates)
        for item in candidates:
            try:
                result = ctx.secret_read(item["secret_key"])
                value = str(result.get("value") or "").strip() if isinstance(result, dict) else ""
                if value:
                    return value
            except Exception as exc:
                self.try_log(ctx, "warn", "订阅 token 读取失败", {"token_id": item.get("id"), "error": str(exc)})
        return ""

    def previous_live_status(self, ctx, subscription):
        value = storage_value(ctx.storage_get(f"live-status:{subscription['id']}"), 0)
        try:
            return int(value)
        except (TypeError, ValueError):
            return 0

    def seen_update(self, ctx, subscription, update):
        key = self.update_key(subscription, update)
        return bool(storage_value(ctx.storage_get(key), False))

    def mark_seen(self, ctx, subscription, update):
        ctx.storage_set(self.update_key(subscription, update), True)
        if update.get("service") == "live":
            ctx.storage_set(f"live-status:{subscription['id']}", int(update.get("live_status") or 0))

    def update_key(self, subscription, update):
        return f"seen:{subscription['id']}:{update.get('service')}:{update.get('id')}"

    def push_update(self, ctx, subscription, update):
        render_data = build_render_data(subscription, update)
        result = ctx.render_image(
            "bilibili-update",
            render_data,
            theme="default",
            output="png",
            fallback_text=build_fallback_text(render_data),
        )
        image_path = str(result.get("image_path") or "").strip()
        if not image_path:
            self.try_log(ctx, "warn", "订阅图片生成结果缺少图片路径")
            return
        ctx.send_message(
            [{
                "type": "image",
                "data": {"file": image_path},
            }],
            target_type=subscription["target_type"],
            target_id=subscription["target_id"],
        )


def service_enabled(subscription, service):
    services = set(subscription.get("services") or ["all"])
    return "all" in services or service in services


def build_status_text(settings):
    tokens = settings.get("tokens") or []
    subscriptions = settings.get("subscriptions") or []
    enabled_tokens = sum(1 for item in tokens if item.get("enabled", True))
    enabled_subscriptions = sum(1 for item in subscriptions if item.get("enabled", True))
    return "\n".join([
        "订阅中心",
        f"状态：{'启用' if settings.get('enabled', True) else '停用'}",
        f"订阅：{enabled_subscriptions}/{len(subscriptions)}",
        f"Token：{enabled_tokens}/{len(tokens)}",
        f"轮询：{settings.get('poll_cron') or '*/5 * * * *'}",
    ])


if __name__ == "__main__":
    SubscriptionHubPlugin().run()
