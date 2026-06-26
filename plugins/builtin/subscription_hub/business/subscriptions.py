from .platforms import platform_name, subject_text
from .services import services_text, subscription_id_for


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


def find_subscription(settings, platform, uid, target):
    subscription_id = subscription_id_for(platform, uid, target["target_type"], target["target_id"])
    return next((
        item for item in settings.get("subscriptions") or []
        if item.get("id") == subscription_id
        or (
            item.get("platform") == platform
            and str(item.get("uid") or "").strip() == str(uid or "").strip()
            and item.get("target_type") == target["target_type"]
            and item.get("target_id") == target["target_id"]
        )
    ), None)


def find_subscription_by_name(settings, platform, name, target):
    text = str(name or "").strip()
    if not text:
        return None
    return next((
        item for item in settings.get("subscriptions") or []
        if item.get("platform") == platform
        and item.get("target_type") == target["target_type"]
        and item.get("target_id") == target["target_id"]
        and str(item.get("name") or "").strip() == text
    ), None)


def find_bilibili_subscription(settings, uid, target):
    return find_subscription(settings, "bilibili", uid, target)


def find_bilibili_subscription_by_name(settings, name, target):
    return find_subscription_by_name(settings, "bilibili", name, target)


def user_label(user):
    return subject_text(user, user.get("platform") or "bilibili")


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
        item_platform = item.get("platform") or "bilibili"
        subject = subject_text(item, item_platform)
        lines.append(f"{target_label} {item.get('target_id')} · {platform_name(item_platform)} {subject} · {services_text(item.get('services'), item_platform)} · 订阅人：{subscribers_text(item)}")
    return "\n".join(lines)


def subscribers_text(subscription):
    names = []
    for item in subscription.get("subscribers") or []:
        text = str(item.get("nickname") or item.get("id") or "").strip()
        if text:
            names.append(text)
    return "、".join(names) or "未记录"


def build_status_text(settings):
    subscriptions = settings.get("subscriptions") or []
    enabled_subscriptions = sum(1 for item in subscriptions if item.get("enabled", True))
    return "\n".join([
        "订阅中心",
        f"状态：{'启用' if settings.get('enabled', True) else '停用'}",
        f"订阅：{enabled_subscriptions}/{len(subscriptions)}",
        "平台：Bilibili、微博、抖音、网易云音乐",
        "检查：订阅中心插件定时检查，支持手动立即检查",
        "账号：Web 三方账号页面管理平台 Cookie",
    ])
