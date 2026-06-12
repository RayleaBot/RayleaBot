#!/usr/bin/env python3
"""Built-in subscription hub plugin for RayleaBot."""

import copy
import os
import sys

PLUGIN_DIR = os.path.dirname(__file__)
sys.path.insert(0, PLUGIN_DIR)
sys.path.insert(0, os.path.join(PLUGIN_DIR, "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command, event_handler
from rayleabot.protocol import ActionError

from bilibili import (
    LIVE_URL,
    live_room_info_url,
    live_status_entry,
    looks_like_preview_url,
    dynamic_detail_url,
    opus_detail_url,
    parse_preview_url,
    preview_update_from_live_document,
    preview_update_from_opus_document,
    preview_update_from_video_document,
    video_view_url,
)
from hub.commands import (
    BILIBILI_SEARCH_UP_USAGE,
    SUBSCRIBE_BILIBILI_USAGE,
    UNSUBSCRIBE_BILIBILI_USAGE,
    add_bilibili_subscription,
    parse_bilibili_command_args,
    remove_bilibili_subscription,
    search_bilibili_users,
)
from hub.events import normalize_bilibili_event_payload, subscription_matches_event
from hub.http_utils import is_http_permission_error, preview_response_document
from hub.preview import (
    first_arg,
    parse_preview_service,
    preview_request_headers,
    preview_subscription,
    sample_subscription,
    sample_update,
)
from rendering import build_fallback_text, build_render_data
from hub.settings import SETTINGS_KEYS, merge_settings, normalize_settings
from hub.subscriptions import build_status_text, current_target, format_subscription_list


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
        self.subscribe(
            "message.group",
            "message.private",
            "config.changed",
            "bilibili.live.started",
            "bilibili.live.ended",
            "bilibili.dynamic.published",
        )
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

    @command("立即检查订阅")
    def handle_manual_check(self, ctx):
        settings = self.load_settings(ctx, force=True)
        if not settings["enabled"]:
            ctx.send_text("订阅中心未启用。")
            ctx.send_result({"handled": True, "skipped": "disabled"})
            return
        ctx.send_text("Bilibili 事件源负责实时检查。请在 Web 的三方监控页面查看连接、运行受限和备用检查状态。")
        ctx.send_result({"handled": True, "sent": 0})

    @command("预览订阅卡片")
    def handle_preview_card(self, ctx):
        argument = first_arg(ctx.args)
        preview_ref = parse_preview_url(argument) if argument else None
        if preview_ref:
            result = self.preview_update_from_link(ctx, preview_ref)
            if not result.get("ok"):
                ctx.send_text(result.get("message") or "Bilibili 链接预览失败。")
                ctx.send_result({"handled": True, "sent": 0})
                return
            self.send_preview_update(ctx, result["update"], real_preview=True)
            return
        if looks_like_preview_url(argument):
            ctx.send_text("暂不支持这个 Bilibili 链接。")
            ctx.send_result({"handled": True, "sent": 0})
            return
        service = parse_preview_service(ctx.args)
        self.send_preview_update(ctx, sample_update(service))

    def send_preview_update(self, ctx, update, real_preview=False):
        subscription = preview_subscription(ctx, update) if real_preview else sample_subscription(ctx)
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
            ctx.send_text("订阅卡片预览生成失败。")
            ctx.send_result({"handled": True, "sent": 0})
            return
        ctx.send_message([{
            "type": "image",
            "data": {"file": image_path},
        }])

    def preview_update_from_link(self, ctx, preview_ref):
        headers = preview_request_headers(preview_ref)
        try:
            if preview_ref["kind"] == "video":
                response = ctx.http_request("GET", video_view_url(preview_ref["bvid"]), headers=headers, timeout_seconds=30)
                document = preview_response_document(response, "视频")
                if isinstance(document, dict):
                    return preview_update_from_video_document(document, preview_ref["url"])
                return {"ok": False, "message": document}
            if preview_ref["kind"] == "opus":
                response = ctx.http_request("GET", opus_detail_url(preview_ref["opus_id"]), headers=headers, timeout_seconds=30)
                document = preview_response_document(response, "动态")
                if isinstance(document, dict):
                    return preview_update_from_opus_document(document, preview_ref["url"])
                return {"ok": False, "message": document}
            if preview_ref["kind"] == "dynamic":
                response = ctx.http_request("GET", dynamic_detail_url(preview_ref["dynamic_id"]), headers=headers, timeout_seconds=30)
                document = preview_response_document(response, "动态")
                if isinstance(document, dict):
                    return preview_update_from_opus_document(document, preview_ref["url"])
                return {"ok": False, "message": document}
            if preview_ref["kind"] == "live":
                response = ctx.http_request("GET", live_room_info_url(preview_ref["room_id"]), headers=headers, timeout_seconds=30)
                document = preview_response_document(response, "直播间")
                if isinstance(document, dict):
                    status_document = self.preview_live_status_document(ctx, document, headers)
                    return preview_update_from_live_document(document, preview_ref["url"], preview_ref["room_id"], status_document)
                return {"ok": False, "message": document}
        except ActionError as exc:
            if is_http_permission_error(exc):
                return {"ok": False, "message": "Bilibili 链接预览失败：请授予订阅中心 HTTP 请求权限，并重载插件后再试。"}
            return {"ok": False, "message": "Bilibili 链接预览失败。"}
        except Exception as exc:
            self.try_log(ctx, "warn", "Bilibili 链接预览失败", {"error": str(exc)})
            return {"ok": False, "message": "Bilibili 链接预览失败。"}
        return {"ok": False, "message": "暂不支持这个 Bilibili 链接。"}

    def preview_live_status_document(self, ctx, document, headers):
        data = document.get("data") if isinstance(document, dict) else {}
        uid = str(data.get("uid") or "").strip() if isinstance(data, dict) else ""
        if not uid:
            return None
        try:
            response = ctx.http_request("GET", LIVE_URL.format(uid=uid), headers=headers, timeout_seconds=30)
        except Exception:
            return None
        status_document = preview_response_document(response, "直播间")
        if not isinstance(status_document, dict):
            return None
        return status_document if live_status_entry(status_document, uid) else None

    def save_settings(self, ctx, settings):
        ctx.config_write({key: settings[key] for key in SETTINGS_KEYS if key in settings})

    @event_handler("config.changed")
    def handle_config_changed(self, ctx):
        self.load_settings(ctx, force=True)
        ctx.send_result({"handled": True})

    @event_handler("bilibili.live.started")
    def handle_bilibili_live_started(self, ctx):
        self.handle_bilibili_event(ctx)

    @event_handler("bilibili.live.ended")
    def handle_bilibili_live_ended(self, ctx):
        self.handle_bilibili_event(ctx)

    @event_handler("bilibili.dynamic.published")
    def handle_bilibili_dynamic_published(self, ctx):
        self.handle_bilibili_event(ctx)

    def handle_bilibili_event(self, ctx):
        settings = self.load_settings(ctx, force=True)
        if not settings["enabled"]:
            ctx.send_result({"handled": True, "skipped": "disabled"})
            return
        payload = ctx.payload.get("bilibili") if isinstance(getattr(ctx, "payload", None), dict) else None
        update = normalize_bilibili_event_payload(payload)
        if not update:
            ctx.send_result({"handled": True, "sent": 0})
            return
        sent = 0
        for subscription in settings["subscriptions"]:
            if not subscription.get("enabled", True):
                continue
            if not subscription_matches_event(subscription, update):
                continue
            if self.seen_update(ctx, subscription, update):
                continue
            prepared = self.prepare_push_update(ctx, subscription, update)
            if prepared:
                self.send_prepared_update(ctx, prepared)
                sent += 1
        ctx.send_result({"handled": True, "sent": sent})

    def seen_update(self, ctx, subscription, update):
        key = self.update_key(subscription, update)
        return bool(storage_value(ctx.storage_get(key), False))

    def mark_seen(self, ctx, subscription, update):
        ctx.storage_set(self.update_key(subscription, update), True)

    def update_key(self, subscription, update):
        return f"seen:{subscription['id']}:{update.get('service')}:{update.get('id')}"

    def prepare_push_update(self, ctx, subscription, update):
        prepared_update = self.prepare_render_update(ctx, update)
        render_data = build_render_data(subscription, prepared_update)
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
            return None
        self.mark_seen(ctx, subscription, update)
        return {
            "image_path": image_path,
            "target_type": subscription["target_type"],
            "target_id": subscription["target_id"],
        }

    def prepare_render_update(self, ctx, update):
        prepared = copy.deepcopy(update)
        if prepared.get("service") != "repost" or isinstance(prepared.get("original"), dict):
            return prepared
        preview_ref = parse_preview_url(prepared.get("url"))
        if not preview_ref or preview_ref.get("kind") not in {"opus", "dynamic"}:
            return prepared
        result = self.preview_update_from_link(ctx, preview_ref)
        if result.get("ok") and isinstance(result.get("update"), dict):
            detailed = result["update"]
            if isinstance(detailed.get("original"), dict):
                prepared["original"] = detailed["original"]
            if not str(prepared.get("summary_html") or "").strip() and str(detailed.get("summary_html") or "").strip():
                prepared["summary_html"] = detailed["summary_html"]
            if not str(prepared.get("summary") or "").strip() and str(detailed.get("summary") or "").strip():
                prepared["summary"] = detailed["summary"]
        return prepared

    def send_prepared_update(self, ctx, prepared):
        ctx.send_message(
            [{
                "type": "image",
                "data": {"file": prepared["image_path"]},
            }],
            target_type=prepared["target_type"],
            target_id=prepared["target_id"],
        )



if __name__ == "__main__":
    SubscriptionHubPlugin().run()
