"""Plugin management page action handlers."""

from rayleabot import event_handler

from business.commands import resolve_bilibili_user
from business.subject_inputs import subject_id_from_input


def management_user_from_result(result):
    user = {
        "uid": str(result.get("uid") or "").strip(),
        "name": str(result.get("name") or "").strip(),
        "avatar_url": str(result.get("avatar_url") or "").strip(),
    }
    fans = result.get("fans")
    if isinstance(fans, int) and fans > 0:
        user["fans"] = fans
    return user


class ManagementActionFeature:
    @event_handler("management.action")
    def handle_management_action(self, ctx):
        payload = ctx.payload if isinstance(getattr(ctx, "payload", None), dict) else {}
        action = str(payload.get("action") or "").strip()
        action_payload = payload.get("payload") if isinstance(payload.get("payload"), dict) else {}
        if action == "subscription.check_now":
            settings = self.load_settings(ctx, force=True)
            result = self.check_subscriptions(ctx, settings)
            ctx.send_result(result)
            return
        if action == "subscription.resolve_user":
            settings = self.load_settings(ctx, force=True)
            ctx.send_result(self.resolve_management_user(settings, ctx, action_payload))
            return
        ctx.send_result({"handled": False, "message": "未知订阅中心管理动作。"})

    def resolve_management_user(self, settings, ctx, payload):
        platform = str(payload.get("platform") or "bilibili").strip()
        query = str(payload.get("query") or "").strip()
        if not query:
            return {"platform": platform, "query": query, "exact": False, "candidates": [], "message": "请填写账号信息。"}
        if platform == "bilibili":
            result = resolve_bilibili_user(settings, ctx, query)
            if result.get("ok"):
                user = management_user_from_result(result)
                return {"platform": platform, "query": query, "exact": True, "user": user, "candidates": [user]}
            return {"platform": platform, "query": query, "exact": False, "candidates": [], "message": result.get("message") or "没有找到这个 Bilibili 用户。"}
        user = self.resolve_basic_platform_user(platform, query)
        return {"platform": platform, "query": query, "exact": True, "user": user, "candidates": [user]}

    def resolve_basic_platform_user(self, platform, query):
        uid = subject_id_from_input(platform, query) or query.strip()
        return {"uid": uid, "name": uid, "avatar_url": ""}
