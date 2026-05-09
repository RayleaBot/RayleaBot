import json
from html import unescape


DYNAMIC_URL = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space?host_mid={uid}&timezone_offset=-480&features=itemOpusStyle"
LIVE_URL = "https://api.live.bilibili.com/room/v1/Room/get_status_info_by_uids?uids[]={uid}"


def build_cookie_headers(token):
    headers = {
        "User-Agent": "Mozilla/5.0 RayleaBot SubscriptionHub/0.1",
        "Referer": "https://www.bilibili.com/",
    }
    token = str(token or "").strip()
    if token:
        headers["Cookie"] = token
    return headers


def parse_json_response(response):
    if not isinstance(response, dict):
        return {}
    body = response.get("body_text")
    if not isinstance(body, str) or not body.strip():
        return {}
    try:
        return json.loads(body)
    except json.JSONDecodeError:
        return {}


def dynamic_updates(document):
    data = document.get("data") if isinstance(document, dict) else {}
    items = data.get("items") if isinstance(data, dict) else []
    if not isinstance(items, list):
        return []
    updates = []
    for item in items:
        update = normalize_dynamic_item(item)
        if update:
            updates.append(update)
    return updates


def normalize_dynamic_item(item):
    if not isinstance(item, dict):
        return None
    basic = item.get("basic") if isinstance(item.get("basic"), dict) else {}
    modules = item.get("modules") if isinstance(item.get("modules"), dict) else {}
    module_author = modules.get("module_author") if isinstance(modules.get("module_author"), dict) else {}
    module_dynamic = modules.get("module_dynamic") if isinstance(modules.get("module_dynamic"), dict) else {}
    desc = module_dynamic.get("desc") if isinstance(module_dynamic.get("desc"), dict) else {}
    major = module_dynamic.get("major") if isinstance(module_dynamic.get("major"), dict) else {}

    dynamic_id = str(item.get("id_str") or item.get("id") or "").strip()
    service = service_for_item(basic, major)
    title = title_for_item(major, desc, service)
    summary = summary_for_item(desc, major)
    url = jump_url_for_item(basic, major, dynamic_id)
    created_at = str(module_author.get("pub_time") or module_author.get("pub_ts") or "").strip()
    author = {
        "name": str(module_author.get("name") or "").strip(),
        "avatar": str(module_author.get("face") or "").strip(),
    }
    if not dynamic_id or not title:
        return None
    return {
        "id": dynamic_id,
        "service": service,
        "title": title,
        "summary": summary,
        "url": url,
        "created_at": created_at,
        "author": author,
    }


def service_for_item(basic, major):
    major_type = str(major.get("type") or "").upper() if isinstance(major, dict) else ""
    comment_type = str(basic.get("comment_type") or "").strip() if isinstance(basic, dict) else ""
    if comment_type == "17":
        return "repost"
    if major_type == "MAJOR_TYPE_ARCHIVE":
        return "video"
    if major_type == "MAJOR_TYPE_ARTICLE":
        return "article"
    if major_type in {"MAJOR_TYPE_DRAW", "MAJOR_TYPE_OPUS", "MAJOR_TYPE_PGC", "MAJOR_TYPE_COMMON"}:
        return "image_text"
    return "image_text"


def title_for_item(major, desc, service):
    if isinstance(major, dict):
        for section_name in ("archive", "article", "opus", "draw", "common"):
            section = major.get(section_name)
            if isinstance(section, dict):
                for key in ("title", "desc", "summary"):
                    text = clean_text(section.get(key))
                    if text:
                        return text
    text = clean_text(desc.get("text") if isinstance(desc, dict) else "")
    if text:
        return text[:40]
    labels = {
        "video": "发布了新视频",
        "article": "发布了新文章",
        "repost": "转发了动态",
        "image_text": "发布了新动态",
    }
    return labels.get(service, "发布了新内容")


def summary_for_item(desc, major):
    text = clean_text(desc.get("text") if isinstance(desc, dict) else "")
    if text:
        return text[:180]
    if isinstance(major, dict):
        for section_name in ("archive", "article", "opus", "draw", "common"):
            section = major.get(section_name)
            if isinstance(section, dict):
                text = clean_text(section.get("desc") or section.get("summary"))
                if text:
                    return text[:180]
    return ""


def jump_url_for_item(basic, major, dynamic_id):
    if isinstance(basic, dict):
        jump_url = str(basic.get("jump_url") or "").strip()
        if jump_url:
            return jump_url if jump_url.startswith("http") else "https:" + jump_url
    if isinstance(major, dict):
        for section_name in ("archive", "article", "opus", "common"):
            section = major.get(section_name)
            if isinstance(section, dict):
                jump_url = str(section.get("jump_url") or "").strip()
                if jump_url:
                    return jump_url if jump_url.startswith("http") else "https:" + jump_url
    return f"https://t.bilibili.com/{dynamic_id}"


def live_update(document, uid):
    data = document.get("data") if isinstance(document, dict) else {}
    entry = data.get(str(uid)) if isinstance(data, dict) else None
    if not isinstance(entry, dict):
        return None
    live_status = int(entry.get("live_status") or 0)
    room_id = str(entry.get("room_id") or "").strip()
    title = clean_text(entry.get("title")) or "直播间状态更新"
    return {
        "id": f"live-{uid}-{live_status}-{room_id}",
        "service": "live",
        "title": title,
        "summary": "直播中" if live_status == 1 else "未开播",
        "url": str(entry.get("url") or entry.get("link") or "").strip(),
        "created_at": "",
        "author": {
            "name": clean_text(entry.get("uname")) or str(uid),
            "avatar": str(entry.get("face") or "").strip(),
        },
        "live_status": live_status,
    }


def clean_text(value):
    text = unescape(str(value or "")).strip()
    return " ".join(text.split())
