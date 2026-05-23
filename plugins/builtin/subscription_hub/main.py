#!/usr/bin/env python3
"""Built-in subscription hub plugin for RayleaBot."""

import copy
import os
import random
import sys
import time

PLUGIN_DIR = os.path.dirname(__file__)
sys.path.insert(0, PLUGIN_DIR)
sys.path.insert(0, os.path.join(PLUGIN_DIR, "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command, event_handler
from rayleabot.protocol import ActionError

from bilibili import (
    DYNAMIC_URL,
    LIVE_URL,
    NAV_URL,
    build_cookie_headers,
    dynamic_updates,
    live_update,
    normalize_user_info,
    normalize_user_search,
    parse_json_response,
    user_info_url,
    user_search_url,
)
from rendering import build_fallback_text, build_render_data
from settings import SETTINGS_KEYS, merge_settings, normalize_settings


DEFAULT_SETTINGS_PATH = os.path.join(PLUGIN_DIR, "default_config.json")
SUBSCRIBE_BILIBILI_USAGE = "用法：/订阅b站推送 [直播|视频|图文|文章|转发] UID或昵称；类型可选，不填表示全部类型。"
UNSUBSCRIBE_BILIBILI_USAGE = "用法：/取消b站推送 [直播|视频|图文|文章|转发] UID或昵称；类型可选，不填表示全部类型。"


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
        self.ensure_scheduler_if_needed(ctx, settings)
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
            self.ensure_scheduler(ctx, settings)
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
            self.ensure_scheduler(ctx, settings)
        ctx.send_text(result["message"])
        ctx.send_result({"handled": True})

    @command("订阅列表")
    def handle_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        self.ensure_scheduler_if_needed(ctx, settings)
        ctx.send_text(format_subscription_list(settings, current_target(ctx), platform=None, title="订阅列表"))
        ctx.send_result({"handled": True})

    @command("b站订阅列表")
    def handle_bilibili_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        self.ensure_scheduler_if_needed(ctx, settings)
        ctx.send_text(format_subscription_list(settings, current_target(ctx), platform="bilibili", title="Bilibili 订阅列表"))
        ctx.send_result({"handled": True})

    @command("全部订阅列表")
    def handle_all_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        self.ensure_scheduler_if_needed(ctx, settings)
        ctx.send_text(format_subscription_list(settings, None, platform=None, title="全部订阅列表"))
        ctx.send_result({"handled": True})

    @command("全部b站订阅列表")
    def handle_all_bilibili_subscription_list(self, ctx):
        settings = self.load_settings(ctx)
        self.ensure_scheduler_if_needed(ctx, settings)
        ctx.send_text(format_subscription_list(settings, None, platform="bilibili", title="全部 Bilibili 订阅列表"))
        ctx.send_result({"handled": True})

    @command("立即检查订阅")
    def handle_manual_check(self, ctx):
        settings = self.load_settings(ctx, force=True)
        self.ensure_scheduler_if_needed(ctx, settings)
        if not settings["enabled"]:
            ctx.send_text("订阅中心未启用。")
            ctx.send_result({"handled": True, "skipped": "disabled"})
            return
        scope = parse_manual_check_scope(ctx.args)
        target = None if scope == "all" else current_target(ctx)
        prepared = self.prepare_next_update(ctx, settings, target=target)
        if prepared:
            self.send_prepared_update(ctx, prepared)
            return
        ctx.send_text("当前没有可推送的订阅更新。")
        ctx.send_result({"handled": True, "sent": 0})

    @command("预览订阅卡片")
    def handle_preview_card(self, ctx):
        service = parse_preview_service(ctx.args)
        render_data = build_render_data(sample_subscription(ctx), sample_update(service))
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

    def save_settings(self, ctx, settings):
        ctx.config_write({key: settings[key] for key in SETTINGS_KEYS if key in settings})

    @event_handler("config.changed")
    def handle_config_changed(self, ctx):
        settings = self.load_settings(ctx, force=True)
        self.ensure_scheduler_if_needed(ctx, settings)
        ctx.send_result({"handled": True})

    @event_handler("scheduler.trigger")
    def handle_scheduler_trigger(self, ctx):
        settings = self.load_settings(ctx, force=True)
        self.ensure_scheduler_if_needed(ctx, settings)
        if not settings["enabled"]:
            ctx.send_result({"handled": True, "skipped": "disabled"})
            return
        prepared = self.prepare_next_update(ctx, settings)
        if prepared:
            self.send_prepared_update(ctx, prepared)
            return
        ctx.send_result({"handled": True, "sent": 0})

    def ensure_scheduler_if_needed(self, ctx, settings):
        if settings.get("enabled", True) and has_enabled_subscriptions(settings):
            self.ensure_scheduler(ctx, settings)

    def ensure_scheduler(self, ctx, settings):
        task_id = "subscription-hub-poll"
        cron = settings["poll_cron"]
        try:
            ctx.scheduler_create(
                task_id,
                cron,
                payload={"kind": "subscription_poll"},
            )
        except Exception as exc:
            self.try_log(ctx, "warn", "订阅轮询任务注册失败", {
                "task_id": task_id,
                "cron": cron,
                "error": str(exc),
            })

    def prepare_next_update(self, ctx, settings, target=None):
        for subscription in settings["subscriptions"]:
            if not subscription.get("enabled", True):
                continue
            if target and (subscription.get("target_type") != target["target_type"] or subscription.get("target_id") != target["target_id"]):
                continue
            if subscription.get("platform") != "bilibili":
                continue
            prepared = self.prepare_bilibili_subscription_update(ctx, settings, subscription)
            if prepared:
                return prepared
        return None

    def prepare_bilibili_subscription_update(self, ctx, settings, subscription):
        token = self.choose_token(ctx, settings)
        if not token:
            return None
        headers = build_cookie_headers(token, subscription["uid"])
        if not self.bilibili_cookie_available(ctx, settings, headers):
            return None
        updates = []
        services = set(subscription.get("services") or ["all"])
        if "all" in services or any(service in services for service in {"video", "image_text", "article", "repost"}):
            response = ctx.http_request(
                "GET",
                DYNAMIC_URL.format(uid=subscription["uid"]),
                headers=headers,
                timeout_seconds=settings["poll_timeout_seconds"],
            )
            document = self.bilibili_response_document(ctx, response, subscription["uid"], "dynamic")
            if document:
                updates.extend(self.filter_dynamic_updates(ctx, settings, subscription, dynamic_updates(document)))
        if "all" in services or "live" in services:
            response = ctx.http_request(
                "GET",
                LIVE_URL.format(uid=subscription["uid"]),
                headers=headers,
                timeout_seconds=settings["poll_timeout_seconds"],
            )
            document = self.bilibili_response_document(ctx, response, subscription["uid"], "live")
            if document:
                update = live_update(document, subscription["uid"])
                if update and (update.get("live_status") == 1 or self.previous_live_status(ctx, subscription) == 1):
                    updates.append(update)

        for update in sorted(updates, key=lambda item: int(item.get("pub_ts") or 0)):
            if not service_enabled(subscription, update.get("service")):
                continue
            if self.seen_update(ctx, subscription, update):
                continue
            prepared = self.prepare_push_update(ctx, subscription, update)
            if prepared:
                return prepared
        return None

    def filter_dynamic_updates(self, ctx, settings, subscription, updates):
        dynamic_updates_only = [item for item in updates if isinstance(item, dict) and item.get("pub_ts")]
        if not dynamic_updates_only:
            return []
        latest_ts = max(int(item.get("pub_ts") or 0) for item in dynamic_updates_only)
        baseline_key = self.dynamic_baseline_key(subscription)
        baseline = self.dynamic_baseline(ctx, subscription)
        if baseline <= 0:
            ctx.storage_set(baseline_key, latest_ts)
            return []
        now = int(time.time())
        window = int(settings.get("dynamic_time_range_seconds") or 7200)
        result = []
        for update in dynamic_updates_only:
            pub_ts = int(update.get("pub_ts") or 0)
            if update.get("is_pinned"):
                continue
            if pub_ts <= baseline:
                continue
            if now - pub_ts > window:
                continue
            result.append(update)
        return result

    def bilibili_cookie_available(self, ctx, settings, headers):
        response = ctx.http_request(
            "GET",
            NAV_URL,
            headers=headers,
            timeout_seconds=settings["poll_timeout_seconds"],
        )
        document = self.bilibili_response_document(ctx, response, "", "cookie")
        if not document:
            return False
        data = document.get("data") if isinstance(document, dict) else {}
        if not isinstance(data, dict) or data.get("isLogin") is not True:
            self.try_log(ctx, "warn", "Bilibili Cookie 无效或已过期")
            return False
        return True

    def bilibili_response_document(self, ctx, response, uid, endpoint):
        status_code = response.get("status_code") if isinstance(response, dict) else None
        if not isinstance(status_code, int) or status_code < 200 or status_code >= 300:
            if status_code == 412:
                self.try_log(ctx, "warn", "Bilibili 风控拦截", {
                    "uid": uid,
                    "endpoint": endpoint,
                    "status_code": status_code,
                })
                return None
            self.try_log(ctx, "warn", "Bilibili 订阅读取失败", {
                "uid": uid,
                "endpoint": endpoint,
                "status_code": status_code,
            })
            return None

        document = parse_json_response(response)
        if not document:
            self.try_log(ctx, "warn", "Bilibili 订阅响应解析失败", {
                "uid": uid,
                "endpoint": endpoint,
                "status_code": status_code,
            })
            return None

        code = document.get("code")
        if code != 0:
            self.try_log(ctx, "warn", "Bilibili 订阅读取失败", {
                "uid": uid,
                "endpoint": endpoint,
                "status_code": status_code,
                "bilibili_code": code,
                "bilibili_message": str(document.get("message") or document.get("msg") or "").strip(),
            })
            return None
        return document

    def choose_token(self, ctx, settings):
        candidates = [item for item in settings["tokens"] if item.get("enabled", True)]
        if not candidates:
            self.try_log(ctx, "warn", "Bilibili Cookie 未配置或未启用")
            return ""
        random.shuffle(candidates)
        for item in candidates:
            try:
                result = ctx.secret_read(item["secret_key"])
                value = str(result.get("value") or "").strip() if isinstance(result, dict) else ""
                if value:
                    return value
            except Exception as exc:
                self.try_log(ctx, "warn", "Bilibili Cookie 读取失败", {"cookie_id": item.get("id"), "error": str(exc)})
        self.try_log(ctx, "warn", "Bilibili Cookie 未读取到可用值")
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
        if update.get("service") != "live" and update.get("pub_ts"):
            current = self.dynamic_baseline(ctx, subscription)
            next_value = max(current, int(update.get("pub_ts") or 0))
            ctx.storage_set(self.dynamic_baseline_key(subscription), next_value)
        if update.get("service") == "live":
            ctx.storage_set(f"live-status:{subscription['id']}", int(update.get("live_status") or 0))

    def update_key(self, subscription, update):
        return f"seen:{subscription['id']}:{update.get('service')}:{update.get('id')}"

    def dynamic_baseline_key(self, subscription):
        return f"dynamic-baseline:{subscription['id']}"

    def dynamic_baseline(self, ctx, subscription):
        value = storage_value(ctx.storage_get(self.dynamic_baseline_key(subscription)), 0)
        try:
            return int(value)
        except (TypeError, ValueError):
            return 0

    def prepare_push_update(self, ctx, subscription, update):
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
            return None
        self.mark_seen(ctx, subscription, update)
        return {
            "image_path": image_path,
            "target_type": subscription["target_type"],
            "target_id": subscription["target_id"],
        }

    def send_prepared_update(self, ctx, prepared):
        ctx.send_message(
            [{
                "type": "image",
                "data": {"file": prepared["image_path"]},
            }],
            target_type=prepared["target_type"],
            target_id=prepared["target_id"],
        )


def service_enabled(subscription, service):
    services = set(subscription.get("services") or ["all"])
    return "all" in services or service in services


def parse_manual_check_scope(args):
    values = [str(item or "").strip() for item in args or [] if str(item or "").strip()]
    return "all" if values and values[0] == "全部" else "current"


def parse_preview_service(args):
    values = [str(item or "").strip() for item in args or [] if str(item or "").strip()]
    if not values:
        return "video"
    return normalize_service_token(values[0]) or "video"


def has_enabled_subscriptions(settings):
    for subscription in settings.get("subscriptions") or []:
        if subscription.get("enabled", True):
            return True
    return False


SERVICE_ALIASES = {
    "全部": "all",
    "全量": "all",
    "所有": "all",
    "直播": "live",
    "视频": "video",
    "图文": "image_text",
    "动态": "image_text",
    "文章": "article",
    "专栏": "article",
    "转发": "repost",
    "live": "live",
    "video": "video",
    "image_text": "image_text",
    "article": "article",
    "repost": "repost",
    "all": "all",
}

SERVICE_NAMES = {
    "all": "全部",
    "live": "直播",
    "video": "视频",
    "image_text": "图文",
    "article": "文章",
    "repost": "转发",
}


def sample_subscription(ctx):
    target = current_target(ctx)
    return {
        "id": f"preview-bilibili-{target['target_type']}-{target['target_id'] or 'current'}",
        "platform": "bilibili",
        "uid": "3546659356389007",
        "name": "RayleaBot 示例账号",
        "target_type": target["target_type"],
        "target_id": target["target_id"],
        "services": ["all"],
        "subscribers": [current_subscriber(ctx)],
        "enabled": True,
    }


def sample_update(service):
    now = int(time.time())
    base = {
        "id": f"preview-{service}",
        "service": service,
        "category": SERVICE_NAMES.get(service, "视频"),
        "author": {"name": "RayleaBot 示例账号"},
        "pub_ts": now,
        "created_at": time.strftime("%Y-%m-%d %H:%M", time.localtime(now)),
        "url": "https://t.bilibili.com/100000000000000001",
        "images": [{"url": "https://i0.hdslb.com/bfs/archive/sample-cover.jpg"}],
    }
    if service == "live":
        return {
            **base,
            "id": "preview-live",
            "title": "直播间已开播",
            "summary": "正在直播：RayleaBot 订阅中心调试示例。",
            "live_status": 1,
            "url": "https://live.bilibili.com/123456",
        }
    if service == "image_text":
        return {
            **base,
            "title": "图文动态示例",
            "summary": "这里展示图文动态正文、图片九宫格、订阅对象和订阅人信息。",
            "images": [
                {"url": "https://i0.hdslb.com/bfs/new_dyn/sample-1.jpg"},
                {"url": "https://i0.hdslb.com/bfs/new_dyn/sample-2.jpg"},
                {"url": "https://i0.hdslb.com/bfs/new_dyn/sample-3.jpg"},
            ],
        }
    if service == "article":
        return {
            **base,
            "title": "专栏文章示例",
            "summary": "文章摘要会显示在卡片正文区域，封面图显示在图片区域。",
            "url": "https://www.bilibili.com/read/cv12345678",
        }
    if service == "repost":
        return {
            **base,
            "title": "转发动态示例",
            "summary": "转发评论会显示在主卡片正文中。",
            "original": {
                "id": "preview-original",
                "service": "video",
                "category": "视频",
                "title": "原动态视频标题",
                "summary": "原动态摘要会以内嵌卡片显示，包含原作者、正文、图片和链接。",
                "author": {"name": "原动态作者"},
                "images": [{"url": "https://i0.hdslb.com/bfs/archive/original-cover.jpg"}],
                "url": "https://www.bilibili.com/video/BV1RayleaBot",
                "created_at": time.strftime("%Y-%m-%d %H:%M", time.localtime(now - 300)),
            },
        }
    return {
        **base,
        "service": "video",
        "category": "视频",
        "title": "新视频示例",
        "summary": "视频简介会被整理为摘要，推送图会显示封面、链接、订阅对象和订阅人。",
        "url": "https://www.bilibili.com/video/BV1RayleaBot",
    }


def add_bilibili_subscription(settings, ctx):
    parsed = parse_bilibili_command_args(ctx.args)
    if parsed["error"] or not parsed["query"]:
        return {"ok": False, "message": SUBSCRIBE_BILIBILI_USAGE}
    target = current_target(ctx)
    if not target["target_id"]:
        return {"ok": False, "message": "当前会话无法绑定订阅目标。"}
    user = resolve_bilibili_user(settings, ctx, parsed["query"])
    if not user["ok"]:
        return {"ok": False, "message": user["message"]}

    subscriptions = list(settings.get("subscriptions") or [])
    subscription = find_bilibili_subscription({"subscriptions": subscriptions}, user["uid"], target)
    if not subscription:
        subscription_id = subscription_id_for("bilibili", user["uid"], target["target_type"], target["target_id"])
        subscription = {
            "id": subscription_id,
            "platform": "bilibili",
            "uid": user["uid"],
            "name": user["name"],
            "target_type": target["target_type"],
            "target_id": target["target_id"],
            "services": [],
            "subscribers": [],
            "enabled": True,
        }
        subscriptions.append(subscription)

    subscription["platform"] = "bilibili"
    subscription["uid"] = user["uid"]
    subscription["name"] = user["name"]
    subscription["target_type"] = target["target_type"]
    subscription["target_id"] = target["target_id"]
    subscription["enabled"] = True
    subscription["services"] = merge_services(subscription.get("services"), parsed["services"])
    subscription["subscribers"] = merge_subscriber(subscription.get("subscribers"), current_subscriber(ctx))
    settings["subscriptions"] = subscriptions
    return {"ok": True, "message": f"已订阅 Bilibili {user_label(user)}：{services_text(subscription['services'])}"}


def remove_bilibili_subscription(settings, ctx):
    parsed = parse_bilibili_command_args(ctx.args)
    if parsed["error"] or not parsed["query"]:
        return {"ok": False, "message": UNSUBSCRIBE_BILIBILI_USAGE}
    target = current_target(ctx)
    user = resolve_bilibili_user_for_removal(settings, ctx, parsed["query"], target)
    if not user["ok"]:
        return {"ok": False, "message": user["message"]}
    subscriptions = list(settings.get("subscriptions") or [])
    subscription = find_bilibili_subscription({"subscriptions": subscriptions}, user["uid"], target)
    if not subscription:
        return {"ok": False, "message": f"当前会话没有订阅 Bilibili {user_label(user)}。"}

    remaining = remove_services(subscription.get("services"), parsed["services"])
    if remaining:
        subscription["services"] = remaining
        message = f"已取消 Bilibili {user_label(user)}：{services_text(parsed['services'])}"
    else:
        subscriptions = [item for item in subscriptions if item is not subscription]
        message = f"已取消 Bilibili {user_label(user)} 的当前会话订阅。"
    settings["subscriptions"] = subscriptions
    return {"ok": True, "message": message}


def parse_bilibili_command_args(args):
    values = [str(item or "").strip() for item in args or [] if str(item or "").strip()]
    service = "all"
    query = ""
    error = False
    if len(values) == 1:
        query = values[0]
    elif len(values) >= 2:
        service = normalize_service_token(values[0])
        query = values[1]
        error = not service
    if not service and not error:
        service = "all"
    uid = digits(query)
    return {"services": [service] if service else [], "uid": uid, "query": query, "error": error}


def resolve_bilibili_user(settings, ctx, query):
    text = str(query or "").strip()
    if not text:
        return {"ok": False, "message": "请填写 Bilibili UID 或昵称。"}
    token = first_available_token(settings, ctx)
    headers = build_cookie_headers(token, text if text.isdigit() else None)
    timeout_seconds = int(settings.get("poll_timeout_seconds") or 12)
    try:
        if text.isdigit():
            response = ctx.http_request("GET", user_info_url(text), headers=headers, timeout_seconds=timeout_seconds)
            result = normalize_user_info(parse_json_response(response))
        else:
            response = ctx.http_request("GET", user_search_url(text), headers=headers, timeout_seconds=timeout_seconds)
            result = normalize_user_search(parse_json_response(response), text)
    except ActionError as exc:
        if is_http_permission_error(exc):
            return {"ok": False, "message": "Bilibili 用户信息读取失败：请授予订阅中心 HTTP 请求权限，并重载插件后再试。"}
        return {"ok": False, "message": "Bilibili 用户信息读取失败。"}
    except Exception:
        return {"ok": False, "message": "Bilibili 用户信息读取失败。"}
    if result.get("ok"):
        return result
    return {"ok": False, "message": result.get("message") or "Bilibili 用户信息读取失败。"}


def is_http_permission_error(exc):
    code = str(getattr(exc, "code", "") or "").lower()
    message = str(exc or "").lower()
    details = str(getattr(exc, "details", "") or "").lower()
    combined = " ".join([code, message, details])
    return "permission" in combined or "scope" in combined or "granted" in combined


def resolve_bilibili_user_for_removal(settings, ctx, query, target):
    text = str(query or "").strip()
    if text.isdigit():
        subscription = find_bilibili_subscription(settings, text, target)
        if subscription:
            return {
                "ok": True,
                "uid": text,
                "name": str(subscription.get("name") or text).strip() or text,
            }
    else:
        subscription = find_bilibili_subscription_by_name(settings, text, target)
        if subscription:
            uid = str(subscription.get("uid") or "").strip()
            if uid:
                return {
                    "ok": True,
                    "uid": uid,
                    "name": str(subscription.get("name") or uid).strip() or uid,
                }
    return resolve_bilibili_user(settings, ctx, text)


def find_bilibili_subscription(settings, uid, target):
    subscription_id = subscription_id_for("bilibili", uid, target["target_type"], target["target_id"])
    return next((
        item for item in settings.get("subscriptions") or []
        if item.get("id") == subscription_id
        or (
            item.get("platform") == "bilibili"
            and str(item.get("uid") or "").strip() == str(uid or "").strip()
            and item.get("target_type") == target["target_type"]
            and item.get("target_id") == target["target_id"]
        )
    ), None)


def find_bilibili_subscription_by_name(settings, name, target):
    text = str(name or "").strip()
    if not text:
        return None
    return next((
        item for item in settings.get("subscriptions") or []
        if item.get("platform") == "bilibili"
        and item.get("target_type") == target["target_type"]
        and item.get("target_id") == target["target_id"]
        and str(item.get("name") or "").strip() == text
    ), None)


def first_available_token(settings, ctx):
    for item in settings.get("tokens") or []:
        if item.get("enabled", True) is False:
            continue
        try:
            result = ctx.secret_read(item["secret_key"])
            value = str(result.get("value") or "").strip() if isinstance(result, dict) else ""
            if value:
                return value
        except Exception:
            continue
    return ""


def user_label(user):
    name = str(user.get("name") or "").strip()
    uid = str(user.get("uid") or "").strip()
    return f"{name}（UID {uid}）" if name and uid and name != uid else uid or name


def normalize_service_token(value):
    return SERVICE_ALIASES.get(str(value or "").strip().lower()) or SERVICE_ALIASES.get(str(value or "").strip())


def current_target(ctx):
    return {
        "target_type": "private" if ctx.target_type == "private" else "group",
        "target_id": str(ctx.target_id or "").strip(),
    }


def current_subscriber(ctx):
    actor = ctx.actor or {}
    subscriber_id = str(actor.get("id") or "").strip()
    nickname = str(actor.get("nickname") or subscriber_id).strip()
    return {"id": subscriber_id, "nickname": nickname or subscriber_id}


def subscription_id_for(platform, uid, target_type, target_id):
    return f"{platform}-{uid}-{target_type}-{target_id}"


def merge_services(existing, incoming):
    current = [service for service in existing or [] if service in SERVICE_NAMES]
    if "all" in current or "all" in incoming:
        return ["all"]
    result = []
    for service in current + incoming:
        if service in SERVICE_NAMES and service not in result:
            result.append(service)
    return result or ["all"]


def remove_services(existing, removing):
    current = [service for service in existing or ["all"] if service in SERVICE_NAMES]
    if "all" in removing:
        return []
    if "all" in current:
        current = ["live", "video", "image_text", "article", "repost"]
    return [service for service in current if service not in removing]


def merge_subscriber(existing, subscriber):
    items = [item for item in existing or [] if isinstance(item, dict) and str(item.get("id") or "").strip()]
    if not subscriber["id"]:
        return items
    for item in items:
        if str(item.get("id") or "") == subscriber["id"]:
            item["nickname"] = subscriber["nickname"]
            return items
    items.append(subscriber)
    return items


def format_subscription_list(settings, target, platform=None, title="订阅列表"):
    items = []
    for subscription in settings.get("subscriptions") or []:
        if platform and subscription.get("platform") != platform:
            continue
        if target and (subscription.get("target_type") != target["target_type"] or subscription.get("target_id") != target["target_id"]):
            continue
        items.append(subscription)
    if not items:
        return f"{title}\n当前没有订阅。"
    lines = [title]
    for item in items:
        target_label = "私聊" if item.get("target_type") == "private" else "群聊"
        name = str(item.get("name") or item.get("uid") or "").strip()
        uid = str(item.get("uid") or "").strip()
        subject = f"{name}（UID {uid}）" if name and uid and name != uid else uid or name
        lines.append(f"{target_label} {item.get('target_id')} · Bilibili {subject} · {services_text(item.get('services'))} · 订阅人：{subscribers_text(item)}")
    return "\n".join(lines)


def services_text(services):
    values = services or ["all"]
    return "、".join(SERVICE_NAMES.get(service, service) for service in values)


def subscribers_text(subscription):
    names = []
    for item in subscription.get("subscribers") or []:
        text = str(item.get("nickname") or item.get("id") or "").strip()
        if text:
            names.append(text)
    return "、".join(names) or "未记录"


def digits(value):
    text = str(value or "").strip()
    return text if text.isdigit() else ""


def build_status_text(settings):
    tokens = settings.get("tokens") or []
    subscriptions = settings.get("subscriptions") or []
    enabled_tokens = sum(1 for item in tokens if item.get("enabled", True))
    enabled_subscriptions = sum(1 for item in subscriptions if item.get("enabled", True))
    return "\n".join([
        "订阅中心",
        f"状态：{'启用' if settings.get('enabled', True) else '停用'}",
        f"订阅：{enabled_subscriptions}/{len(subscriptions)}",
        f"Cookie：{enabled_tokens}/{len(tokens)}",
        f"轮询：{settings.get('poll_cron') or '*/5 * * * *'}",
        f"动态窗口：{settings.get('dynamic_time_range_seconds') or 7200} 秒",
    ])


if __name__ == "__main__":
    SubscriptionHubPlugin().run()
