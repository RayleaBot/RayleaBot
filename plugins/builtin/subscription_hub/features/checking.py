"""Subscription checking and update delivery feature."""

import copy
from datetime import datetime, timezone

from rayleabot_runtime import command, event_handler

from business.events import subscription_matches_event
from features.rendering import build_fallback_text, build_render_data
from platforms.bilibili import (
    parse_preview_url,
)
from platforms.bilibili.source import BilibiliSourceClient


def storage_value(result, fallback=None):
    if isinstance(result, dict):
        if result.get("exists") and "value" in result:
            return result.get("value")
        if "value" in result:
            return result.get("value")
    return fallback


class SubscriptionCheckFeature:
    @command("立即检查订阅")
    def handle_manual_check(self, ctx):
        settings = self.load_settings(ctx, force=True)
        result = self.check_subscriptions(ctx, settings)
        ctx.send_text(self.check_summary_text(result))
        ctx.send_result({"handled": True, **result})

    @event_handler("scheduler.trigger")
    def handle_scheduler_trigger(self, ctx):
        payload = ctx.payload if isinstance(getattr(ctx, "payload", None), dict) else {}
        job_payload = payload.get("payload") if isinstance(payload.get("payload"), dict) else {}
        action = str(payload.get("action") or job_payload.get("action") or "").strip()
        if action and action != "check_subscriptions":
            ctx.send_result({"handled": False})
            return
        settings = self.load_settings(ctx, force=True)
        result = self.check_subscriptions(ctx, settings)
        self.log_check_result(ctx, result)
        ctx.send_result({"handled": True, **result})

    def check_subscriptions(self, ctx, settings):
        result = {"checked": 0, "sent": 0, "errors": [], "degraded": False}
        if not settings["enabled"]:
            result["skipped"] = "disabled"
            return result
        subscriptions = [
            item for item in settings.get("subscriptions", [])
            if item.get("enabled", True) and item.get("platform") == "bilibili"
        ]
        if not subscriptions:
            result["skipped"] = "no_bilibili_subscriptions"
            return result
        source_result = BilibiliSourceClient(ctx).poll(subscriptions)
        result["checked"] = source_result["checked"]
        result["errors"].extend(source_result["errors"])
        result["source"] = {
            "accounts": source_result["account_count"],
            "dynamic_ok": source_result["dynamic_ok"],
            "live_ok": source_result["live_ok"],
        }
        updates = source_result["updates"]
        render_update_cache = {}
        for subscription in subscriptions:
            dynamic_initialized = self.dynamic_source_initialized(ctx, subscription)
            for update in updates:
                if not subscription_matches_event(subscription, update):
                    continue
                if update.get("service") != "live" and not dynamic_initialized:
                    self.mark_seen(ctx, subscription, update)
                    continue
                if self.seen_update(ctx, subscription, update):
                    continue
                prepared = self.prepare_push_update(ctx, subscription, update, render_update_cache)
                if prepared and self.send_prepared_update(ctx, subscription, update, prepared, result["errors"]):
                    result["sent"] += 1
            if source_result["dynamic_ok"] and self.subscription_uses_dynamic(subscription) and not dynamic_initialized:
                self.mark_dynamic_source_initialized(ctx, subscription)
        result["degraded"] = bool(result["errors"])
        self.remember_check_result(ctx, result)
        return result

    def check_summary_text(self, result):
        if result.get("skipped") == "disabled":
            return "订阅中心未启用。"
        if result.get("skipped") == "no_bilibili_subscriptions":
            return "没有可检查的 Bilibili 订阅。"
        errors = result.get("errors") or []
        line = f"订阅检查完成：检查 {int(result.get('checked') or 0)} 个订阅账号，推送 {int(result.get('sent') or 0)} 条更新。"
        if errors:
            return line + "\n" + "\n".join(f"- {item}" for item in errors[:3])
        return line

    def log_check_result(self, ctx, result):
        errors = result.get("errors") or []
        if errors:
            self.try_log(ctx, "warn", "Bilibili 订阅源检查降级", {
                "checked": int(result.get("checked") or 0),
                "sent": int(result.get("sent") or 0),
                "errors": errors[:3],
            })
            return
        self.try_log(ctx, "info", "Bilibili 订阅源检查完成", {
            "checked": int(result.get("checked") or 0),
            "sent": int(result.get("sent") or 0),
        })

    def remember_check_result(self, ctx, result):
        try:
            ctx.storage_set("source:bilibili:last_result", {
                "checked_at": datetime.now(timezone.utc).isoformat(),
                "checked": int(result.get("checked") or 0),
                "sent": int(result.get("sent") or 0),
                "degraded": bool(result.get("errors")),
                "errors": list((result.get("errors") or [])[:3]),
                "source": copy.deepcopy(result.get("source") or {}),
            })
        except Exception as exc:
            self.try_log(ctx, "warn", "Bilibili 订阅源状态保存失败", {"error": str(exc)})

    def subscription_uses_dynamic(self, subscription):
        services = {str(value or "").strip() for value in subscription.get("services") or []}
        return "all" in services or bool(services - {"live"})

    def dynamic_source_initialized(self, ctx, subscription):
        if not self.subscription_uses_dynamic(subscription):
            return True
        return bool(storage_value(ctx.storage_get(self.dynamic_source_key(subscription)), False))

    def mark_dynamic_source_initialized(self, ctx, subscription):
        ctx.storage_set(self.dynamic_source_key(subscription), True)

    def dynamic_source_key(self, subscription):
        return f"source:bilibili:dynamic:initialized:{subscription['id']}"

    def seen_update(self, ctx, subscription, update):
        key = self.update_key(subscription, update)
        return bool(storage_value(ctx.storage_get(key), False))

    def mark_seen(self, ctx, subscription, update):
        ctx.storage_set(self.update_key(subscription, update), True)

    def update_key(self, subscription, update):
        return f"seen:{subscription['id']}:{update.get('service')}:{update.get('id')}"

    def prepare_push_update(self, ctx, subscription, update, render_update_cache=None):
        service = str(update.get("service") or "").strip()
        update_id = str(update.get("id") or "").strip()
        cache_key = f"{service}:{update_id}" if service and update_id else ""
        if isinstance(render_update_cache, dict) and cache_key and cache_key in render_update_cache:
            prepared_update = render_update_cache[cache_key]
        else:
            prepared_update = self.prepare_render_update(ctx, update)
            if isinstance(render_update_cache, dict) and cache_key:
                render_update_cache[cache_key] = prepared_update
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
        return {
            "image_path": image_path,
            "target_type": subscription["target_type"],
            "target_id": subscription["target_id"],
        }

    def prepare_render_update(self, ctx, update):
        prepared = copy.deepcopy(update)
        preview_ref = parse_preview_url(prepared.get("url"))
        if (
            prepared.get("service") == "image_text"
            and not str(prepared.get("summary") or "").strip()
            and not str(prepared.get("summary_html") or "").strip()
            and preview_ref
            and preview_ref.get("kind") in {"opus", "dynamic"}
        ):
            result = self.preview_update_from_link(ctx, preview_ref)
            if result.get("ok") and isinstance(result.get("update"), dict):
                self.merge_missing_image_text_detail(prepared, result["update"])
        if prepared.get("service") != "repost" or isinstance(prepared.get("original"), dict):
            return prepared
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

    def merge_missing_image_text_detail(self, prepared, detailed):
        for key in ("summary", "summary_html"):
            if not str(prepared.get(key) or "").strip() and str(detailed.get(key) or "").strip():
                prepared[key] = detailed[key]
        if not isinstance(prepared.get("topic"), dict) and isinstance(detailed.get("topic"), dict):
            prepared["topic"] = detailed["topic"]
        if not prepared.get("images") and detailed.get("images"):
            prepared["images"] = detailed["images"]

    def send_prepared_update(self, ctx, subscription, update, prepared, errors):
        try:
            ctx.message_send(
                prepared["target_type"],
                prepared["target_id"],
                [{
                    "type": "image",
                    "data": {"file": prepared["image_path"]},
                }],
            )
        except Exception as exc:
            errors.append("Bilibili 订阅推送失败。")
            self.try_log(ctx, "warn", "Bilibili 订阅推送失败", {
                "error": str(exc),
                "subscription_id": subscription.get("id"),
                "target_type": prepared["target_type"],
                "target_id": prepared["target_id"],
            })
            return False
        self.mark_seen(ctx, subscription, update)
        return True
