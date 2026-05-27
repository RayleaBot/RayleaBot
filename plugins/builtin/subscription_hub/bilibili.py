import json
from datetime import datetime, timezone
from html import unescape
from urllib.parse import urlencode


DYNAMIC_URL = "https://api.bilibili.com/x/polymer/web-dynamic/v1/feed/space?host_mid={uid}&timezone_offset=-480&platform=web&web_location=333.1365&features=itemOpusStyle,listOnlyfans,opusBigCover,onlyfansVote,decorationCard,onlyfansAssetsV2,forwardListHidden,ugcDelete"
LIVE_URL = "https://api.live.bilibili.com/room/v1/Room/get_status_info_by_uids?uids[]={uid}"
NAV_URL = "https://api.bilibili.com/x/web-interface/nav"
USER_INFO_URL = "https://api.bilibili.com/x/space/acc/info?mid={uid}&jsonp=jsonp"
USER_SEARCH_URL = "https://api.bilibili.com/x/web-interface/search/type?{query}"


def build_cookie_headers(token, uid=None):
    headers = {
        "Accept": "application/json, text/plain, */*",
        "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
        "Referer": f"https://space.bilibili.com/{uid}/dynamic" if uid else "https://www.bilibili.com/",
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
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


def user_info_url(uid):
    return USER_INFO_URL.format(uid=uid)


def user_search_url(keyword):
    query = urlencode({
        "keyword": keyword,
        "page": 1,
        "search_type": "bili_user",
        "order": "totalrank",
        "pagesize": 5,
    })
    return USER_SEARCH_URL.format(query=query)


def bilibili_document_error(document):
    if not isinstance(document, dict):
        return {"kind": "invalid", "message": "Bilibili 响应格式不正确。"}
    code = document.get("code")
    if code == 0:
        return None
    message = str(document.get("message") or document.get("msg") or "").strip()
    if code == -412:
        return {"kind": "blocked", "message": "Bilibili 请求被风控拦截，请配置可用 Cookie 后再试。"}
    if code == -404:
        return {"kind": "not_found", "message": "没有找到这个 Bilibili 用户。"}
    return {"kind": "api_error", "message": message or "Bilibili 用户信息读取失败。", "code": code}


def normalize_user_info(document):
    error = bilibili_document_error(document)
    if error:
        return {"ok": False, **error}
    data = document.get("data") if isinstance(document, dict) else {}
    if not isinstance(data, dict):
        return {"ok": False, "kind": "invalid", "message": "Bilibili 用户信息格式不正确。"}
    uid = str(data.get("mid") or "").strip()
    name = clean_text(data.get("name") or data.get("uname") or "")
    if not uid.isdigit() or not name:
        return {"ok": False, "kind": "not_found", "message": "没有找到这个 Bilibili 用户。"}
    return {"ok": True, "uid": uid, "name": name}


def normalize_user_search(document, keyword):
    error = bilibili_document_error(document)
    if error:
        return {"ok": False, **error}
    data = document.get("data") if isinstance(document, dict) else {}
    result = data.get("result") if isinstance(data, dict) else []
    if not isinstance(result, list) or not result:
        return {"ok": False, "kind": "not_found", "message": f"没有搜索到 Bilibili 用户：{keyword}"}
    candidates = []
    for item in result:
        if not isinstance(item, dict):
            continue
        uid = str(item.get("mid") or "").strip()
        name = clean_text(item.get("uname") or item.get("name") or "")
        if uid.isdigit() and name:
            candidates.append({"uid": uid, "name": name})
    if not candidates:
        return {"ok": False, "kind": "invalid", "message": "Bilibili 用户搜索结果格式不正确。"}
    keyword_text = clean_text(keyword)
    exact = next((item for item in candidates if item["name"] == keyword_text), None)
    return {"ok": True, **(exact or candidates[0])}


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


def normalize_dynamic_item(item, depth=0):
    if not isinstance(item, dict):
        return None
    basic = item.get("basic") if isinstance(item.get("basic"), dict) else {}
    modules = item.get("modules") if isinstance(item.get("modules"), dict) else {}
    module_author = modules.get("module_author") if isinstance(modules.get("module_author"), dict) else {}
    module_dynamic = modules.get("module_dynamic") if isinstance(modules.get("module_dynamic"), dict) else {}
    module_tag = modules.get("module_tag") if isinstance(modules.get("module_tag"), dict) else {}
    desc = module_dynamic.get("desc") if isinstance(module_dynamic.get("desc"), dict) else {}
    major = module_dynamic.get("major") if isinstance(module_dynamic.get("major"), dict) else {}

    dynamic_id = str(item.get("id_str") or item.get("id") or "").strip()
    item_type = str(item.get("type") or "").strip()
    service = service_for_item(item_type, basic, major)
    author_name = str(module_author.get("name") or "").strip()
    title = title_for_item(major, desc, service, item_type, author_name)
    summary = summary_for_item(desc, major)
    url = jump_url_for_item(basic, major, dynamic_id)
    pub_ts = normalize_pub_ts(module_author.get("pub_ts"))
    created_at = format_pub_time(pub_ts, module_author.get("pub_time"))
    original = normalize_dynamic_item(item.get("orig"), depth + 1) if item_type == "DYNAMIC_TYPE_FORWARD" and depth < 2 else None
    if service == "repost" and original and not summary:
        summary = "转发动态"
    author = {
        "name": author_name,
        "avatar": str(module_author.get("face") or "").strip(),
    }
    if not dynamic_id or not title or not pub_ts:
        return None
    return {
        "id": dynamic_id,
        "type": item_type,
        "service": service,
        "category": category_for_service(service),
        "title": title,
        "summary": summary,
        "url": url,
        "pub_ts": pub_ts,
        "created_at": created_at,
        "author": author,
        "images": image_list_for_item(major, service),
        "is_pinned": clean_text(module_tag.get("text")) == "置顶",
        "original": original,
    }


def service_for_item(item_type, basic, major):
    major_type = str(major.get("type") or "").upper() if isinstance(major, dict) else ""
    comment_type = str(basic.get("comment_type") or "").strip() if isinstance(basic, dict) else ""
    if item_type == "DYNAMIC_TYPE_FORWARD" or comment_type == "17":
        return "repost"
    if item_type == "DYNAMIC_TYPE_AV" or major_type == "MAJOR_TYPE_ARCHIVE":
        return "video"
    if item_type == "DYNAMIC_TYPE_ARTICLE" or major_type == "MAJOR_TYPE_ARTICLE":
        return "article"
    if item_type in {"DYNAMIC_TYPE_DRAW", "DYNAMIC_TYPE_WORD"}:
        return "image_text"
    if major_type in {"MAJOR_TYPE_DRAW", "MAJOR_TYPE_OPUS", "MAJOR_TYPE_PGC", "MAJOR_TYPE_COMMON"}:
        return "image_text"
    return "image_text"


def category_for_service(service):
    return {
        "video": "视频动态",
        "image_text": "图文动态",
        "article": "文章动态",
        "repost": "转发动态",
    }.get(service, "动态")


def title_for_item(major, desc, service, item_type="", author_name=""):
    if service == "repost":
        return "转发动态"
    if isinstance(major, dict):
        for section_name in ("archive", "article", "opus", "common"):
            section = major.get(section_name)
            if isinstance(section, dict):
                for key in ("title",):
                    text = clean_text(section.get(key))
                    if text:
                        return text
    action = default_title_action(service, item_type)
    return f"{author_name} {action}" if author_name else action


def default_title_action(service, item_type=""):
    if service == "video":
        return "发布新视频"
    if service == "article":
        return "发布新文章"
    if service == "repost":
        return "转发动态"
    if item_type == "DYNAMIC_TYPE_WORD":
        return "发布文字动态"
    if service == "image_text":
        return "发布图文动态"
    return "发布新内容"


def summary_for_item(desc, major):
    text = clean_text(desc.get("rich_text_nodes") if isinstance(desc, dict) else "") or clean_text(desc.get("text") if isinstance(desc, dict) else "")
    if text:
        return truncate_text(text, 420)
    if isinstance(major, dict):
        for section_name in ("archive", "article", "opus", "draw", "common"):
            section = major.get(section_name)
            if isinstance(section, dict):
                text = clean_text(section.get("desc") or section.get("summary") or section.get("content"))
                if text:
                    return truncate_text(text, 420)
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


def image_list_for_item(major, service):
    if not isinstance(major, dict):
        return []
    images = []
    if service == "video":
        archive = major.get("archive") if isinstance(major.get("archive"), dict) else {}
        add_image(images, archive.get("cover"))
    elif service == "article":
        article = major.get("article") if isinstance(major.get("article"), dict) else {}
        covers = article.get("covers")
        if isinstance(covers, list):
            for item in covers:
                add_image(images, item)
        else:
            add_image(images, covers)
        opus = major.get("opus") if isinstance(major.get("opus"), dict) else {}
        for item in opus.get("pics") or []:
            add_image(images, item)
    else:
        draw = major.get("draw") if isinstance(major.get("draw"), dict) else {}
        for item in draw.get("items") or []:
            add_image(images, item)
        opus = major.get("opus") if isinstance(major.get("opus"), dict) else {}
        for item in opus.get("pics") or []:
            add_image(images, item)
        common = major.get("common") if isinstance(major.get("common"), dict) else {}
        add_image(images, common.get("cover"))
    return images[:9]


def add_image(images, value):
    if isinstance(value, str):
        url = normalize_url(value)
        if url:
            images.append({"url": url})
        return
    if not isinstance(value, dict):
        return
    url = normalize_url(value.get("url") or value.get("src") or value.get("cover"))
    if not url:
        return
    image = {"url": url}
    for key in ("width", "height"):
        try:
            number = int(value.get(key) or 0)
        except (TypeError, ValueError):
            number = 0
        if number > 0:
            image[key] = number
    images.append(image)


def normalize_url(value):
    text = str(value or "").strip()
    if not text:
        return ""
    return text if text.startswith("http") else "https:" + text


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
    if value is None:
        return ""
    if isinstance(value, dict):
        for key in ("text", "orig_text", "title", "desc", "summary", "content"):
            text = clean_text(value.get(key))
            if text:
                return text
        for key in ("rich_text_nodes", "paragraphs"):
            text = clean_text(value.get(key))
            if text:
                return text
        return ""
    if isinstance(value, list):
        return " ".join(text for text in (clean_text(item) for item in value) if text)
    if not isinstance(value, str):
        value = str(value)
    text = unescape(value).replace("\\r\\n", "\n").replace("\\n", "\n").replace("\\t", " ").strip()
    return "\n".join(" ".join(line.split()) for line in text.splitlines()).strip()


def truncate_text(text, limit):
    text = clean_text(text)
    if len(text) <= limit:
        return text
    return text[:limit].rstrip() + "..."


def normalize_pub_ts(value):
    try:
        number = int(value)
    except (TypeError, ValueError):
        return 0
    return number if number > 0 else 0


def format_pub_time(pub_ts, fallback):
    text = str(fallback or "").strip()
    if pub_ts:
        return datetime.fromtimestamp(pub_ts, tz=timezone.utc).astimezone().strftime("%Y年%m月%d日 %H:%M")
    return text
