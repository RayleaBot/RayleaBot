"""Subscription checking and update delivery feature."""

import copy

from rayleabot import command, event_handler
from rayleabot.protocol import ActionError

from business.events import subscription_matches_event
from business.http_utils import bilibili_response_failure, is_http_capability_error
from business.thirdparty_accounts import read_thirdparty_cookie as read_thirdparty_cookie_value
from features.rendering import build_fallback_text, build_render_data
from platforms.bilibili import (
    DYNAMIC_URL,
    LIVE_URL,
    build_cookie_headers,
    dynamic_updates,
    live_update,
    parse_json_response,
    parse_preview_url,
)


def storage_value(result, fallback=None):
    if isinstance(result, dict):
        if result.get("exists") and "value" in result:
            return result.get("value")
        if "value" in result:
            return result.get("value")
    return fallback


def response_document(response):
    return parse_json_response(response)


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
        if payload.get("job_id") != self.SCHEDULER_TASK_ID and payload.get("action") != "check_subscriptions":
            ctx.send_result({"handled": False})
            return
        settings = self.load_settings(ctx, force=True)
        result = self.check_subscriptions(ctx, settings)
        ctx.send_result({"handled": True, **result})

    def check_subscriptions(self, ctx, settings):
        result = {"checked": 0, "sent": 0, "errors": []}
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
        cookie, cookie_error = self.read_thirdparty_cookie(ctx, "bilibili")
        if not cookie:
            result["errors"].append(cookie_error or "没有可用的 Bilibili 账号 CK。")
            return result
        updates = []
        for uid in sorted({str(item.get("uid") or "").strip() for item in subscriptions if str(item.get("uid") or "").strip()}):
            result["checked"] += 1
            fetched, errors = self.fetch_bilibili_updates(ctx, uid, cookie)
            updates.extend(fetched)
            result["errors"].extend(errors)
        for subscription in settings["subscriptions"]:
            if not subscription.get("enabled", True):
                continue
            for update in updates:
                if not subscription_matches_event(subscription, update):
                    continue
                if self.seen_update(ctx, subscription, update):
                    continue
                prepared = self.prepare_push_update(ctx, subscription, update)
                if prepared:
                    self.send_prepared_update(ctx, prepared)
                    result["sent"] += 1
        return result

    def read_thirdparty_cookie(self, ctx, platform):
        return read_thirdparty_cookie_value(ctx, platform)

    def fetch_bilibili_updates(self, ctx, uid, cookie):
        headers = build_cookie_headers(cookie, uid)
        updates = []
        errors = []
        try:
            response = ctx.http_request("GET", DYNAMIC_URL.format(uid=uid), headers=headers, timeout_seconds=30)
            failure = bilibili_response_failure(response, "Bilibili 动态检查失败", friendly_risk_control=True)
            if failure:
                errors.append(failure)
            else:
                for update in dynamic_updates(response_document(response)):
                    update["uid"] = uid
                    updates.append(update)
        except ActionError as exc:
            if is_http_capability_error(exc):
                errors.append("Bilibili 动态检查失败：请检查订阅中心 manifest 的 http.request 与 capability_parameters.http_hosts，并重载插件后再试。")
            else:
                errors.append("Bilibili 动态检查失败。")
        except Exception:
            errors.append("Bilibili 动态检查失败。")

        try:
            response = ctx.http_request("GET", LIVE_URL.format(uid=uid), headers=headers, timeout_seconds=30)
            failure = bilibili_response_failure(response, "Bilibili 直播检查失败", friendly_risk_control=True)
            if failure:
                errors.append(failure)
            else:
                update = live_update(response_document(response), uid)
                if update:
                    update["uid"] = uid
                    updates.append(update)
        except ActionError as exc:
            if is_http_capability_error(exc):
                errors.append("Bilibili 直播检查失败：请检查订阅中心 manifest 的 http.request 与 capability_parameters.http_hosts，并重载插件后再试。")
            else:
                errors.append("Bilibili 直播检查失败。")
        except Exception:
            errors.append("Bilibili 直播检查失败。")
        return updates, errors

    def check_summary_text(self, result):
        if result.get("skipped") == "disabled":
            return "订阅中心未启用。"
        if result.get("skipped") == "no_bilibili_subscriptions":
            return "没有可检查的 Bilibili 订阅。"
        errors = result.get("errors") or []
        line = f"订阅检查完成：检查 {int(result.get('checked') or 0)} 个账号，推送 {int(result.get('sent') or 0)} 条更新。"
        if errors:
            return line + "\n" + "\n".join(f"- {item}" for item in errors[:3])
        return line

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
