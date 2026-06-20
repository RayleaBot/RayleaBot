from rayleabot.protocol import ActionError

from bilibili import (
    build_cookie_headers,
    normalize_user_info,
    normalize_user_search,
    normalize_user_search_results,
    parse_json_response,
    user_info_url,
    user_search_url,
)

from .http_utils import (
    bilibili_response_failure,
    is_http_capability_error,
    response_details_text,
    sentence_text,
)
from .platforms import platform_name, safe_subject_id, subject_label, subject_text
from .services import digits, merge_services, normalize_service_token, remove_services, services_text
from .subscriptions import (
    current_subscriber,
    current_target,
    find_bilibili_subscription,
    find_bilibili_subscription_by_name,
    find_subscription,
    find_subscription_by_name,
    merge_subscriber,
    subscription_id_for,
    user_label,
)


SUBSCRIBE_BILIBILI_USAGE = "用法：/订阅b站推送 [直播|视频|图文|文章|转发] UID或昵称；类型可选，不填表示全部类型。"
UNSUBSCRIBE_BILIBILI_USAGE = "用法：/取消b站推送 [直播|视频|图文|文章|转发] UID或昵称；类型可选，不填表示全部类型。"
BILIBILI_SEARCH_UP_USAGE = "用法：/b站搜索up UP昵称关键词"

SUBSCRIBE_USAGE_BY_PLATFORM = {
    "weibo": "用法：/订阅微博推送 [微博|图片|视频|转发] UID或主页标识；类型可选，不填表示全部类型。",
    "douyin": "用法：/订阅抖音推送 [视频|图文|直播] 抖音号或主页标识；类型可选，不填表示全部类型。",
    "netease_music": "用法：/订阅网易云音乐推送 [歌曲|专辑|歌单|音乐人] ID或主页标识；类型可选，不填表示全部类型。",
}

UNSUBSCRIBE_USAGE_BY_PLATFORM = {
    "weibo": "用法：/取消微博推送 [微博|图片|视频|转发] UID或主页标识；类型可选，不填表示全部类型。",
    "douyin": "用法：/取消抖音推送 [视频|图文|直播] 抖音号或主页标识；类型可选，不填表示全部类型。",
    "netease_music": "用法：/取消网易云音乐推送 [歌曲|专辑|歌单|音乐人] ID或主页标识；类型可选，不填表示全部类型。",
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


def search_bilibili_users(ctx):
    query = parse_bilibili_search_query(ctx.args)
    if not query:
        return {"ok": False, "count": 0, "message": BILIBILI_SEARCH_UP_USAGE}
    headers = build_cookie_headers("", None)
    response = None
    try:
        response = ctx.http_request("GET", user_search_url(query), headers=headers, timeout_seconds=12)
        failure = bilibili_response_failure(response, "Bilibili UP 搜索失败")
        if failure:
            return {"ok": False, "count": 0, "message": failure}
        document = parse_json_response(response)
        result = normalize_user_search_results(document, query)
    except ActionError as exc:
        if is_http_capability_error(exc):
            return {"ok": False, "count": 0, "message": "Bilibili UP 搜索失败：请检查订阅中心 manifest 的 http.request 与 capability_parameters.http_hosts，并重载插件后再试。"}
        return {"ok": False, "count": 0, "message": "Bilibili UP 搜索失败。"}
    except Exception:
        return {"ok": False, "count": 0, "message": "Bilibili UP 搜索失败。"}
    if not result.get("ok"):
        message = result.get("message") or "Bilibili UP 搜索失败。"
        if response is not None and result.get("kind") != "not_found":
            message = f"{sentence_text(message)}{response_details_text(response)}"
        return {"ok": False, "count": 0, "message": message}
    items = result.get("items") or []
    return {"ok": True, "count": len(items), "message": format_bilibili_user_search_results(query, items)}


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


def add_platform_subscription(settings, ctx, platform):
    parsed = parse_platform_command_args(ctx.args, platform)
    if parsed["error"] or not parsed["query"]:
        return {"ok": False, "message": SUBSCRIBE_USAGE_BY_PLATFORM[platform]}
    target = current_target(ctx)
    if not target["target_id"]:
        return {"ok": False, "message": "当前会话无法绑定订阅目标。"}
    user = resolve_manual_subject(platform, parsed["query"])
    if not user["ok"]:
        return {"ok": False, "message": user["message"]}

    subscriptions = list(settings.get("subscriptions") or [])
    subscription = find_subscription({"subscriptions": subscriptions}, platform, user["uid"], target)
    if not subscription:
        subscription_id = subscription_id_for(platform, user["uid"], target["target_type"], target["target_id"])
        subscription = {
            "id": subscription_id,
            "platform": platform,
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
        subscriptions.append(subscription)

    subscription["platform"] = platform
    subscription["uid"] = user["uid"]
    subscription["name"] = user["name"]
    subscription["target_type"] = target["target_type"]
    subscription["target_id"] = target["target_id"]
    if target.get("target_name"):
        subscription["target_name"] = target["target_name"]
    subscription["enabled"] = True
    subscription["services"] = merge_services(subscription.get("services"), parsed["services"], platform)
    subscription["subscribers"] = merge_subscriber(subscription.get("subscribers"), current_subscriber(ctx))
    settings["subscriptions"] = subscriptions
    return {"ok": True, "message": f"已订阅 {platform_name(platform)} {subject_text(user, platform)}：{services_text(subscription['services'], platform)}"}


def remove_platform_subscription(settings, ctx, platform):
    parsed = parse_platform_command_args(ctx.args, platform)
    if parsed["error"] or not parsed["query"]:
        return {"ok": False, "message": UNSUBSCRIBE_USAGE_BY_PLATFORM[platform]}
    target = current_target(ctx)
    user = resolve_manual_subject_for_removal(settings, platform, parsed["query"], target)
    if not user["ok"]:
        return {"ok": False, "message": user["message"]}
    subscriptions = list(settings.get("subscriptions") or [])
    subscription = find_subscription({"subscriptions": subscriptions}, platform, user["uid"], target)
    if not subscription:
        return {"ok": False, "message": f"当前会话没有订阅 {platform_name(platform)} {subject_text(user, platform)}。"}

    remaining = remove_services(subscription.get("services"), parsed["services"], platform)
    if remaining:
        subscription["services"] = remaining
        message = f"已取消 {platform_name(platform)} {subject_text(user, platform)}：{services_text(parsed['services'], platform)}"
    else:
        subscriptions = [item for item in subscriptions if item is not subscription]
        message = f"已取消 {platform_name(platform)} {subject_text(user, platform)} 的当前会话订阅。"
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


def parse_platform_command_args(args, platform):
    values = [str(item or "").strip() for item in args or [] if str(item or "").strip()]
    service = "all"
    query = ""
    error = False
    if len(values) == 1:
        query = values[0]
    elif len(values) >= 2:
        service = normalize_service_token(values[0], platform)
        query = values[1]
        error = not service
    if not service and not error:
        service = "all"
    uid = safe_subject_id(query, platform)
    return {"services": [service] if service else [], "uid": uid, "query": query, "error": error}


def parse_bilibili_search_query(args):
    values = [str(item or "").strip() for item in args or [] if str(item or "").strip()]
    return " ".join(values).strip()


def format_bilibili_user_search_results(keyword, items):
    lines = [f"Bilibili UP 搜索结果：{keyword}"]
    for index, item in enumerate(items[:5], start=1):
        label = user_label(item)
        fans = int(item.get("fans") or 0)
        suffix = f"｜粉丝 {format_count(fans)}" if fans > 0 else ""
        lines.append(f"{index}. {label}{suffix}")
    return "\n".join(lines)


def format_count(value):
    number = int(value or 0)
    if number >= 10000:
        text = f"{number / 10000:.1f}".rstrip("0").rstrip(".")
        return f"{text}万"
    return str(number)


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
        if is_http_capability_error(exc):
            return {"ok": False, "message": "Bilibili 用户信息读取失败：请检查订阅中心 manifest 的 http.request 与 capability_parameters.http_hosts，并重载插件后再试。"}
        return {"ok": False, "message": "Bilibili 用户信息读取失败。"}
    except Exception:
        return {"ok": False, "message": "Bilibili 用户信息读取失败。"}
    if result.get("ok"):
        return result
    message = result.get("message") or "Bilibili 用户信息读取失败。"
    if "response" in locals():
        message = f"{sentence_text(message)}{response_details_text(response)}"
    return {"ok": False, "message": message}


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


def resolve_manual_subject(platform, query):
    text = str(query or "").strip()
    uid = safe_subject_id(text, platform)
    if not uid:
        return {"ok": False, "message": f"请填写 {platform_name(platform)} {subject_label(platform)} 或主页标识。"}
    return {
        "ok": True,
        "platform": platform,
        "uid": uid,
        "name": text,
    }


def resolve_manual_subject_for_removal(settings, platform, query, target):
    text = str(query or "").strip()
    uid = safe_subject_id(text, platform)
    if uid:
        subscription = find_subscription(settings, platform, uid, target)
        if subscription:
            return {
                "ok": True,
                "platform": platform,
                "uid": uid,
                "name": str(subscription.get("name") or uid).strip() or uid,
            }
    subscription = find_subscription_by_name(settings, platform, text, target)
    if subscription:
        uid = str(subscription.get("uid") or "").strip()
        if uid:
            return {
                "ok": True,
                "platform": platform,
                "uid": uid,
                "name": str(subscription.get("name") or uid).strip() or uid,
            }
    resolved = resolve_manual_subject(platform, text)
    if resolved["ok"]:
        return {"ok": False, "message": f"当前会话没有订阅 {platform_name(platform)} {subject_text(resolved, platform)}。"}
    return resolved
