from urllib.parse import parse_qs

from hub.link_utils import capability_message, first_query_value, json_response, path_parts, plain_text


PLATFORM = {
    "id": "weibo",
    "name": "微博",
    "subject_label": "UID",
    "subscribe_usage": "用法：/订阅微博推送 [微博|图片|视频|转发] UID或主页标识；类型可选，不填表示全部类型。",
    "unsubscribe_usage": "用法：/取消微博推送 [微博|图片|视频|转发] UID或主页标识；类型可选，不填表示全部类型。",
    "services": {
        "all": "全部",
        "post": "微博",
        "image": "图片",
        "video": "视频",
        "repost": "转发",
    },
    "service_aliases": {
        "全部": "all",
        "全量": "all",
        "所有": "all",
        "微博": "post",
        "动态": "post",
        "文字": "post",
        "图片": "image",
        "图文": "image",
        "视频": "video",
        "转发": "repost",
        "post": "post",
        "image": "image",
        "video": "video",
        "repost": "repost",
        "all": "all",
    },
}

LINK_KIND_NAMES = {
    "weibo_status": "微博动态",
}


def parse_link(url, parsed):
    host = parsed.hostname.lower() if parsed.hostname else ""
    if not (host.endswith("weibo.com") or host.endswith("weibo.cn")):
        return None
    parts = path_parts(parsed.path)
    query = parse_qs(parsed.query)
    status_id = first_query_value(query, "id")
    if not status_id and len(parts) >= 2 and parts[0] in {"status", "detail"}:
        status_id = parts[1]
    if not status_id and len(parts) >= 2:
        status_id = parts[1]
    if not status_id:
        return None
    return {
        "platform": "weibo",
        "kind": "weibo_status",
        "id": status_id,
        "url": url,
    }


def resolve_link_preview(ctx, ref):
    from rayleabot.protocol import ActionError

    status_id = str(ref.get("id") or "").strip()
    if not status_id:
        return {}
    try:
        response = ctx.http_request("GET", f"https://m.weibo.cn/statuses/show?id={status_id}", timeout_seconds=12)
    except ActionError as exc:
        return capability_message(exc)
    except Exception:
        return {}
    document = json_response(response)
    data = document.get("data") if isinstance(document, dict) else {}
    if not isinstance(data, dict):
        return {}
    user = data.get("user") if isinstance(data.get("user"), dict) else {}
    result = {}
    title = plain_text(data.get("text"))
    if title:
        result["title"] = title[:120]
    author = str(user.get("screen_name") or "").strip()
    if author:
        result["author"] = author
    return result
