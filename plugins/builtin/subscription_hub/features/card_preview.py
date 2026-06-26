"""Bilibili subscription card preview feature."""

from rayleabot import command
from rayleabot.protocol import ActionError

from business.http_utils import is_http_capability_error, preview_response_document
from business.preview import (
    first_arg,
    parse_preview_service,
    preview_request_headers,
    preview_subscription,
    sample_subscription,
    sample_update,
)
from features.rendering import build_fallback_text, build_render_data
from platforms.bilibili import (
    LIVE_URL,
    dynamic_detail_url,
    live_room_info_url,
    live_status_entry,
    looks_like_preview_url,
    opus_detail_url,
    parse_preview_url,
    preview_update_from_live_document,
    preview_update_from_opus_document,
    preview_update_from_video_document,
    video_view_url,
)


class CardPreviewFeature:
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
            if is_http_capability_error(exc):
                return {"ok": False, "message": "Bilibili 链接预览失败：请检查订阅中心 manifest 的 http.request 与 capability_parameters.http_hosts，并重载插件后再试。"}
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
