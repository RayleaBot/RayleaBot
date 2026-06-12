#!/usr/bin/env python3
"""Built-in subscription hub plugin for RayleaBot."""

import copy
import os
import sys
import time
import base64

PLUGIN_DIR = os.path.dirname(__file__)
sys.path.insert(0, PLUGIN_DIR)
sys.path.insert(0, os.path.join(PLUGIN_DIR, "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, command, event_handler
from rayleabot.protocol import ActionError

from bilibili import (
    LIVE_URL,
    bilibili_document_error,
    build_cookie_headers,
    live_room_info_url,
    live_status_entry,
    looks_like_preview_url,
    normalize_user_info,
    normalize_user_search,
    dynamic_detail_url,
    opus_detail_url,
    parse_json_response,
    parse_preview_url,
    preview_update_from_live_document,
    preview_update_from_opus_document,
    preview_update_from_video_document,
    user_info_url,
    user_search_url,
    video_view_url,
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


def service_enabled(subscription, service):
    services = set(subscription.get("services") or ["all"])
    return "all" in services or service in services


def first_arg(args):
    values = [str(item or "").strip() for item in args or [] if str(item or "").strip()]
    return values[0] if values else ""


def parse_preview_service(args):
    argument = first_arg(args)
    if not argument:
        return "video"
    return normalize_service_token(argument) or "video"


def preview_response_document(response, label):
    failure_label = f"Bilibili {label}预览失败"
    response_failure = bilibili_response_failure(response, failure_label)
    if response_failure:
        return response_failure
    document = parse_json_response(response)
    error = bilibili_document_error(document)
    if error:
        message = error.get("message") or "Bilibili 响应读取失败。"
        if error.get("kind") == "not_found":
            message = f"没有找到这个 Bilibili {label}。"
        return f"{failure_label}：{sentence_text(message)}{response_details_text(response)}"
    return document


def bilibili_response_failure(response, label):
    status_code = response.get("status_code") if isinstance(response, dict) else None
    if not isinstance(status_code, int) or status_code < 200 or status_code >= 300:
        return f"{label}：{response_details_text(response)}"
    if not parse_json_response(response):
        return f"{label}：Bilibili 返回内容不是 JSON。{response_details_text(response)}"
    return None


def preview_request_headers(preview_ref):
    headers = build_cookie_headers("", None)
    referer = str((preview_ref or {}).get("url") or "").strip()
    if referer:
        headers["Referer"] = referer
    return headers


def response_details_text(response):
    status_code = response.get("status_code") if isinstance(response, dict) else None
    body_excerpt = response_body_excerpt(response)
    return f"HTTP {http_status_text(status_code)}{response_excerpt_suffix(body_excerpt)}"


def http_status_text(status_code):
    return str(status_code) if isinstance(status_code, int) else "未知"


def sentence_text(text):
    text = str(text or "").strip()
    if not text:
        return ""
    return text if text.endswith(("。", "！", "？", ".", "!", "?")) else text + "。"


def response_body_excerpt(response, limit=600):
    if not isinstance(response, dict):
        return ""
    body = response.get("body_text")
    if isinstance(body, str):
        text = body
    else:
        body_base64 = response.get("body_base64")
        if not isinstance(body_base64, str) or not body_base64.strip():
            return ""
        try:
            raw = base64.b64decode(body_base64, validate=True)
        except Exception:
            return "[binary response]"
        text = raw.decode("utf-8", errors="replace")
    text = " ".join(str(text or "").split())
    if len(text) <= limit:
        return text
    return text[:limit].rstrip() + "..."


def response_excerpt_suffix(excerpt):
    return f"。响应：{excerpt}" if excerpt else "。"


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


def normalize_bilibili_event_payload(payload):
    if not isinstance(payload, dict):
        return None
    update = copy.deepcopy(payload)
    update_id = str(update.get("id") or "").strip()
    uid = str(update.get("uid") or "").strip()
    service = str(update.get("service") or "").strip()
    if not update_id or not uid or service not in SERVICE_NAMES or service == "all":
        return None

    update["id"] = update_id
    update["uid"] = uid
    update["service"] = service
    update.setdefault("category", SERVICE_NAMES.get(service, service))
    author = update.get("author") if isinstance(update.get("author"), dict) else {}
    if not str(author.get("uid") or "").strip():
        author["uid"] = uid
    if not str(author.get("name") or "").strip():
        author["name"] = uid
    update["author"] = author
    update["images"] = normalize_bilibili_images(update.get("images"))
    topic = normalize_bilibili_topic(update.get("topic"))
    if topic:
        update["topic"] = topic
    elif "topic" in update:
        update.pop("topic", None)
    original = normalize_bilibili_original(update.get("original"))
    if original:
        update["original"] = original
    elif "original" in update:
        update.pop("original", None)
    return update


def normalize_bilibili_original(value):
    if not isinstance(value, dict):
        return None
    original = copy.deepcopy(value)
    update_id = str(original.get("id") or "").strip()
    service = str(original.get("service") or "").strip()
    url = str(original.get("url") or "").strip()
    if not update_id or service not in SERVICE_NAMES or service == "all" or not url:
        return None
    original["id"] = update_id
    original["service"] = service
    original.setdefault("category", SERVICE_NAMES.get(service, service))
    author = original.get("author") if isinstance(original.get("author"), dict) else {}
    if not str(author.get("uid") or "").strip():
        return None
    if not str(author.get("name") or "").strip():
        author["name"] = str(author.get("uid") or "").strip()
    original["author"] = author
    original["images"] = normalize_bilibili_images(original.get("images"))
    topic = normalize_bilibili_topic(original.get("topic"))
    if topic:
        original["topic"] = topic
    elif "topic" in original:
        original.pop("topic", None)
    return original


def normalize_bilibili_topic(value):
    if not isinstance(value, dict):
        return None
    name = str(value.get("name") or "").strip().strip("# \t\r\n")
    if not name:
        return None
    topic = {"name": name}
    try:
        topic_id = int(value.get("id") or 0)
    except (TypeError, ValueError):
        topic_id = 0
    if topic_id > 0:
        topic["id"] = topic_id
    jump_url = str(value.get("jump_url") or "").strip()
    if jump_url:
        topic["jump_url"] = jump_url
    return topic


def normalize_bilibili_images(value):
    images = value if isinstance(value, list) else []
    return [item for item in images if isinstance(item, dict) and str(item.get("url") or "").strip()]


def subscription_matches_event(subscription, update):
    return (
        subscription.get("platform") == "bilibili"
        and str(subscription.get("uid") or "").strip() == str(update.get("uid") or "").strip()
        and service_enabled(subscription, update.get("service"))
    )


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


def preview_subscription(ctx, update):
    target = current_target(ctx)
    author = update.get("author") if isinstance(update.get("author"), dict) else {}
    uid = str(author.get("uid") or "").strip()
    name = str(author.get("name") or "").strip()
    if not uid:
        uid = str(update.get("id") or "preview").strip()
    if not name:
        name = "Bilibili 预览"
    return {
        "id": f"preview-bilibili-{target['target_type']}-{target['target_id'] or 'current'}",
        "platform": "bilibili",
        "uid": uid,
        "name": name,
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
        started_at = time.strftime("%Y-%m-%d %H:%M", time.localtime(now))
        return {
            **base,
            "id": "preview-live",
            "title": "直播间已开播",
            "summary": f"直播中\n开播时间：{started_at}",
            "live_status": 1,
            "live_event": "started",
            "status_label": "直播中",
            "live_started_at": started_at,
            "url": "https://live.bilibili.com/123456",
        }
    if service == "image_text":
        return {
            **base,
            "title": "图文动态示例",
            "summary": "这里展示图文动态正文、图片九宫格、UP 主信息和订阅人身份。",
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
        "summary": "视频简介会被整理为摘要，推送图会显示封面、时长、链接和订阅人。",
        "duration_text": "12:48",
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
        if target.get("target_name"):
            subscription["target_name"] = target["target_name"]
        if user.get("avatar_url"):
            subscription["avatar_url"] = user["avatar_url"]
        subscriptions.append(subscription)

    subscription["platform"] = "bilibili"
    subscription["uid"] = user["uid"]
    subscription["name"] = user["name"]
    subscription["target_type"] = target["target_type"]
    subscription["target_id"] = target["target_id"]
    if target.get("target_name"):
        subscription["target_name"] = target["target_name"]
    if user.get("avatar_url"):
        subscription["avatar_url"] = user["avatar_url"]
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
    headers = build_cookie_headers("", text if text.isdigit() else None)
    timeout_seconds = 12
    try:
        if text.isdigit():
            response = ctx.http_request("GET", user_info_url(text), headers=headers, timeout_seconds=timeout_seconds)
            failure = bilibili_response_failure(response, "Bilibili 用户信息读取失败")
            if failure:
                return {"ok": False, "message": failure}
            document = parse_json_response(response)
            result = normalize_user_info(document)
        else:
            response = ctx.http_request("GET", user_search_url(text), headers=headers, timeout_seconds=timeout_seconds)
            failure = bilibili_response_failure(response, "Bilibili 用户信息读取失败")
            if failure:
                return {"ok": False, "message": failure}
            document = parse_json_response(response)
            result = normalize_user_search(document, text)
    except ActionError as exc:
        if is_http_permission_error(exc):
            return {"ok": False, "message": "Bilibili 用户信息读取失败：请授予订阅中心 HTTP 请求权限，并重载插件后再试。"}
        return {"ok": False, "message": "Bilibili 用户信息读取失败。"}
    except Exception:
        return {"ok": False, "message": "Bilibili 用户信息读取失败。"}
    if result.get("ok"):
        return result
    message = result.get("message") or "Bilibili 用户信息读取失败。"
    if "response" in locals():
        message = f"{sentence_text(message)}{response_details_text(response)}"
    return {"ok": False, "message": message}


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
        return {"ok": False, "message": f"当前会话没有订阅 Bilibili {text}。"}
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


def user_label(user):
    name = str(user.get("name") or "").strip()
    uid = str(user.get("uid") or "").strip()
    return f"{name}（UID {uid}）" if name and uid and name != uid else uid or name


def normalize_service_token(value):
    return SERVICE_ALIASES.get(str(value or "").strip().lower()) or SERVICE_ALIASES.get(str(value or "").strip())


def current_target(ctx):
    target_type = "private" if ctx.target_type == "private" else "group"
    target_id = str(ctx.target_id or "").strip()
    target = getattr(ctx, "target", None)
    target_name = ""
    if isinstance(target, dict):
        target_name = str(target.get("name") or "").strip()
    onebot = ctx.payload.get("onebot") if isinstance(getattr(ctx, "payload", None), dict) else {}
    sender = onebot.get("sender") if isinstance(onebot, dict) and isinstance(onebot.get("sender"), dict) else {}
    actor = ctx.actor or {}
    if not target_name and target_type == "group" and isinstance(onebot, dict):
        target_name = str(onebot.get("group_name") or "").strip()
    if not target_name and target_type == "private":
        target_name = str(actor.get("nickname") or sender.get("nickname") or "").strip()
    result = {
        "target_type": "private" if ctx.target_type == "private" else "group",
        "target_id": target_id,
    }
    if target_name and target_name != target_id:
        result["target_name"] = target_name
    return result


def current_subscriber(ctx):
    actor = ctx.actor or {}
    subscriber_id = str(actor.get("id") or "").strip()
    onebot = ctx.payload.get("onebot") if isinstance(getattr(ctx, "payload", None), dict) else {}
    sender = onebot.get("sender") if isinstance(onebot, dict) and isinstance(onebot.get("sender"), dict) else {}
    if not subscriber_id:
        subscriber_id = str(sender.get("user_id") or onebot.get("user_id") if isinstance(onebot, dict) else "").strip()
    nickname = str(actor.get("nickname") or sender.get("nickname") or subscriber_id).strip()
    group_nickname = str(sender.get("card") or "").strip()
    role = subscriber_role_from_context(ctx, subscriber_id, actor, sender)
    subscriber = {"id": subscriber_id, "nickname": nickname or subscriber_id}
    if group_nickname:
        subscriber["group_nickname"] = group_nickname
    title = str(sender.get("title") or "").strip()
    if title:
        subscriber["title"] = title
    if role:
        subscriber["role"] = role
        subscriber["role_label"] = subscriber_role_label(role)
    if subscriber_id.isdigit():
        subscriber["avatar_url"] = f"https://q1.qlogo.cn/g?b=qq&nk={subscriber_id}&s=100"
    return subscriber


def subscriber_role_from_context(ctx, subscriber_id, actor, sender):
    if subscriber_id and subscriber_id in super_admin_ids_from_context(ctx):
        return "super_admin"
    return normalize_subscriber_role(actor.get("role") or sender.get("role"))


def super_admin_ids_from_context(ctx):
    values = []
    for source in (
        getattr(ctx, "super_admins", None),
        getattr(getattr(ctx, "_plugin", None), "super_admins", None),
    ):
        if callable(source):
            try:
                source = source()
            except Exception:
                source = None
        if isinstance(source, (list, tuple, set)):
            values.extend(source)
    return {str(item).strip() for item in values if str(item).strip()}


def normalize_subscriber_role(value):
    role = str(value or "").strip()
    return role if role in {"super_admin", "owner", "admin", "member"} else ""


def subscriber_role_label(role):
    return {
        "super_admin": "超级管理员",
        "owner": "群主",
        "admin": "管理员",
        "member": "群员",
    }.get(role, "")


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
            item.update(subscriber)
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
    subscriptions = settings.get("subscriptions") or []
    enabled_subscriptions = sum(1 for item in subscriptions if item.get("enabled", True))
    return "\n".join([
        "订阅中心",
        f"状态：{'启用' if settings.get('enabled', True) else '停用'}",
        f"订阅：{enabled_subscriptions}/{len(subscriptions)}",
        "事件源：平台 Bilibili 实时源",
        "账号：Web 三方账号页面管理 Bilibili CK",
    ])


if __name__ == "__main__":
    SubscriptionHubPlugin().run()
