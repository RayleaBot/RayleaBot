import time

from bilibili import build_cookie_headers

from .services import SERVICE_NAMES, normalize_service_token
from .subscriptions import current_subscriber, current_target


def first_arg(args):
    values = [str(item or "").strip() for item in args or [] if str(item or "").strip()]
    return values[0] if values else ""


def parse_preview_service(args):
    argument = first_arg(args)
    if not argument:
        return "video"
    return normalize_service_token(argument) or "video"


def preview_request_headers(preview_ref):
    headers = build_cookie_headers("", None)
    referer = str((preview_ref or {}).get("url") or "").strip()
    if referer:
        headers["Referer"] = referer
    return headers


def sample_subscription(ctx):
    target = current_target(ctx)
    return {
        "id": f"preview-bilibili-{target['target_type']}-{target['target_id'] or 'current'}",
        "platform": "bilibili",
        "uid": "100000000000000009",
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
            "url": "https://www.bilibili.com/read/cv10001",
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
