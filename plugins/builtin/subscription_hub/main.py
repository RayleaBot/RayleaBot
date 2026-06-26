#!/usr/bin/env python3
"""Built-in subscription hub plugin for RayleaBot."""

import copy
import json
import os
import sys

PLUGIN_DIR = os.path.dirname(__file__)
sys.path.insert(0, PLUGIN_DIR)
sys.path.insert(0, os.path.join(PLUGIN_DIR, "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command, event_handler

from business.commands import (
    BILIBILI_SEARCH_UP_USAGE,
    SUBSCRIBE_BILIBILI_USAGE,
    UNSUBSCRIBE_BILIBILI_USAGE,
    add_bilibili_subscription,
    add_platform_subscription,
    parse_bilibili_command_args,
    remove_bilibili_subscription,
    remove_platform_subscription,
    search_bilibili_users,
)
from business.http_utils import preview_response_document
from business.settings import SETTINGS_KEYS, merge_settings, normalize_settings
from business.subscriptions import build_status_text, current_target, format_subscription_list
from features.card_preview import CardPreviewFeature
from features.checking import SubscriptionCheckFeature
from features.management import ManagementActionFeature


DEFAULT_SETTINGS_PATH = os.path.join(PLUGIN_DIR, "default_config.json")
SCHEDULER_TASK_ID = "subscription-hub-check"
SCHEDULER_CRON = "*/10 * * * *"


def load_default_settings(path=DEFAULT_SETTINGS_PATH):
    with open(path, "r", encoding="utf-8") as handle:
        return normalize_settings(json.load(handle))


class SubscriptionHubPlugin(
    CardPreviewFeature,
    SubscriptionCheckFeature,
    ManagementActionFeature,
    RayleaBotPlugin,
):
    SCHEDULER_TASK_ID = SCHEDULER_TASK_ID
    SCHEDULER_CRON = SCHEDULER_CRON

    def __init__(self):
        super().__init__()
        self.subscribe(
            "config.changed",
            "scheduler.trigger",
            "management.action",
        )
        self._default_settings = load_default_settings()
        self._settings = copy.deepcopy(self._default_settings)
        self._settings_loaded = False
        self._scheduler_registered = False

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
        self.ensure_scheduler(ctx)
        return copy.deepcopy(self._settings)

    def ensure_scheduler(self, ctx):
        if self._scheduler_registered:
            return
        try:
            ctx.scheduler_create(
                self.SCHEDULER_TASK_ID,
                self.SCHEDULER_CRON,
                payload={"action": "check_subscriptions"},
                log_label="订阅检查",
            )
            self._scheduler_registered = True
        except Exception as exc:
            self.try_log(ctx, "warn", "订阅检查任务注册失败", {"error": str(exc)})

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

    @command("订阅b站推送")
    def handle_subscribe_bilibili(self, ctx):
        settings = self.load_settings(ctx)
        result = add_bilibili_subscription(settings, ctx)
        if result["ok"]:
            self.save_settings(ctx, settings)
            self._settings = copy.deepcopy(settings)
            self._settings_loaded = True
        ctx.send_text(result["message"])
        ctx.send_result({"handled": True})

    @command("取消b站推送")
    def handle_unsubscribe_bilibili(self, ctx):
        settings = self.load_settings(ctx)
        result = remove_bilibili_subscription(settings, ctx)
        if result["ok"]:
            self.save_settings(ctx, settings)
            self._settings = copy.deepcopy(settings)
            self._settings_loaded = True
        ctx.send_text(result["message"])
        ctx.send_result({"handled": True})

    @command("订阅微博推送")
    def handle_subscribe_weibo(self, ctx):
        self.handle_platform_subscription_command(ctx, "weibo", add=True)

    @command("取消微博推送")
    def handle_unsubscribe_weibo(self, ctx):
        self.handle_platform_subscription_command(ctx, "weibo", add=False)

    @command("订阅抖音推送")
    def handle_subscribe_douyin(self, ctx):
        self.handle_platform_subscription_command(ctx, "douyin", add=True)

    @command("取消抖音推送")
    def handle_unsubscribe_douyin(self, ctx):
        self.handle_platform_subscription_command(ctx, "douyin", add=False)

    @command("订阅网易云音乐推送")
    def handle_subscribe_netease_music(self, ctx):
        self.handle_platform_subscription_command(ctx, "netease_music", add=True)

    @command("取消网易云音乐推送")
    def handle_unsubscribe_netease_music(self, ctx):
        self.handle_platform_subscription_command(ctx, "netease_music", add=False)

    def handle_platform_subscription_command(self, ctx, platform, add):
        settings = self.load_settings(ctx)
        result = add_platform_subscription(settings, ctx, platform) if add else remove_platform_subscription(settings, ctx, platform)
        if result["ok"]:
            self.save_settings(ctx, settings)
            self._settings = copy.deepcopy(settings)
            self._settings_loaded = True
        ctx.send_text(result["message"])
        ctx.send_result({"handled": True})

    @command("b站搜索up", aliases=["b站搜索UP", "B站搜索up", "B站搜索UP"])
    def handle_bilibili_user_search(self, ctx):
        result = search_bilibili_users(ctx)
        ctx.send_text(result["message"])
        ctx.send_result({"handled": True, "count": result.get("count", 0)})

    @command("订阅列表")
    def handle_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(format_subscription_list(settings, current_target(ctx), platform=None, title="订阅列表"))
        ctx.send_result({"handled": True})

    @command("b站订阅列表")
    def handle_bilibili_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(format_subscription_list(settings, current_target(ctx), platform="bilibili", title="Bilibili 订阅列表"))
        ctx.send_result({"handled": True})

    @command("微博订阅列表")
    def handle_weibo_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(format_subscription_list(settings, current_target(ctx), platform="weibo", title="微博订阅列表"))
        ctx.send_result({"handled": True})

    @command("抖音订阅列表")
    def handle_douyin_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(format_subscription_list(settings, current_target(ctx), platform="douyin", title="抖音订阅列表"))
        ctx.send_result({"handled": True})

    @command("网易云音乐订阅列表")
    def handle_netease_music_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(format_subscription_list(settings, current_target(ctx), platform="netease_music", title="网易云音乐订阅列表"))
        ctx.send_result({"handled": True})

    @command("全部订阅列表")
    def handle_all_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(format_subscription_list(settings, None, platform=None, title="全部订阅列表"))
        ctx.send_result({"handled": True})

    @command("全部b站订阅列表")
    def handle_all_bilibili_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(format_subscription_list(settings, None, platform="bilibili", title="全部 Bilibili 订阅列表"))
        ctx.send_result({"handled": True})

    @command("全部微博订阅列表")
    def handle_all_weibo_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(format_subscription_list(settings, None, platform="weibo", title="全部微博订阅列表"))
        ctx.send_result({"handled": True})

    @command("全部抖音订阅列表")
    def handle_all_douyin_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(format_subscription_list(settings, None, platform="douyin", title="全部抖音订阅列表"))
        ctx.send_result({"handled": True})

    @command("全部网易云音乐订阅列表")
    def handle_all_netease_music_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        ctx.send_text(format_subscription_list(settings, None, platform="netease_music", title="全部网易云音乐订阅列表"))
        ctx.send_result({"handled": True})

    def save_settings(self, ctx, settings):
        ctx.config_write({key: settings[key] for key in SETTINGS_KEYS if key in settings})

    @event_handler("config.changed")
    def handle_config_changed(self, ctx):
        self.load_settings(ctx, force=True)
        ctx.send_result({"handled": True})


if __name__ == "__main__":
    SubscriptionHubPlugin().run()
