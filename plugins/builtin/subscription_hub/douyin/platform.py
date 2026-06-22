from hub.link_utils import capability_message, html_title, path_parts


PLATFORM = {
    "id": "douyin",
    "name": "抖音",
    "subject_label": "抖音号",
    "subscribe_usage": "用法：/订阅抖音推送 [视频|图文|直播] 抖音号或主页标识；类型可选，不填表示全部类型。",
    "unsubscribe_usage": "用法：/取消抖音推送 [视频|图文|直播] 抖音号或主页标识；类型可选，不填表示全部类型。",
    "services": {
        "all": "全部",
        "video": "视频",
        "image_text": "图文",
        "live": "直播",
    },
    "service_aliases": {
        "全部": "all",
        "全量": "all",
        "所有": "all",
        "视频": "video",
        "图文": "image_text",
        "图片": "image_text",
        "直播": "live",
        "video": "video",
        "image_text": "image_text",
        "live": "live",
        "all": "all",
    },
}

LINK_KIND_NAMES = {
    "douyin_video": "抖音视频",
    "douyin_note": "抖音图文",
}


def parse_link(url, parsed):
    host = parsed.hostname.lower() if parsed.hostname else ""
    if not (host.endswith("douyin.com") or host.endswith("iesdouyin.com") or host.endswith("amemv.com")):
        return None
    parts = path_parts(parsed.path)
    for marker, kind in (("video", "douyin_video"), ("note", "douyin_note")):
        if marker in parts:
            index = parts.index(marker)
            if len(parts) > index + 1:
                return {
                    "platform": "douyin",
                    "kind": kind,
                    "id": parts[index + 1],
                    "url": url,
                }
    return None


def resolve_link_preview(ctx, ref):
    from rayleabot.protocol import ActionError

    url = str(ref.get("url") or "").strip()
    if not url:
        return {}
    try:
        response = ctx.http_request("GET", url, headers={"User-Agent": "Mozilla/5.0"}, timeout_seconds=12)
    except ActionError as exc:
        return capability_message(exc)
    except Exception:
        return {}
    title = html_title(response)
    return {"title": title[:120]} if title else {}
