import html
import json
import re
from urllib.parse import parse_qs, urlparse, urlunparse

from rayleabot.protocol import ActionError

from .http_utils import is_http_capability_error
from .platforms import platform_name


URL_PATTERN = re.compile(r"https?://[^\s<>\"]+")

LINK_KIND_NAMES = {
    "weibo_status": "微博动态",
    "douyin_video": "抖音视频",
    "douyin_note": "抖音图文",
    "douyin_short": "抖音短链接",
    "netease_song": "网易云音乐歌曲",
    "netease_album": "网易云音乐专辑",
    "netease_playlist": "网易云音乐歌单",
    "netease_artist": "网易云音乐人",
    "netease_short": "网易云音乐短链接",
}


def parse_supported_link(text):
    for raw_url in URL_PATTERN.findall(str(text or "")):
        url = cleanup_url(raw_url)
        ref = parse_one_url(url)
        if ref:
            return ref
    return None


def cleanup_url(value):
    text = str(value or "").strip().rstrip("。），,)")
    parsed = urlparse(text)
    return urlunparse((parsed.scheme, parsed.netloc, parsed.path, "", parsed.query, parsed.fragment))


def parse_one_url(url):
    parsed = urlparse(url)
    host = parsed.hostname.lower() if parsed.hostname else ""
    if host.endswith("weibo.com") or host.endswith("weibo.cn"):
        return parse_weibo_url(url, parsed)
    if host.endswith("douyin.com") or host.endswith("iesdouyin.com") or host.endswith("amemv.com"):
        return parse_douyin_url(url, parsed)
    if host.endswith("music.163.com") or host.endswith("163cn.tv"):
        return parse_netease_url(url, parsed)
    return None


def parse_weibo_url(url, parsed):
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


def parse_douyin_url(url, parsed):
    parts = path_parts(parsed.path)
    if parsed.hostname and parsed.hostname.lower().startswith("v."):
        return {
            "platform": "douyin",
            "kind": "douyin_short",
            "id": parts[0] if parts else "",
            "url": url,
        }
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


def parse_netease_url(url, parsed):
    fragment_path, _, fragment_query = str(parsed.fragment or "").partition("?")
    parts = path_parts(parsed.path) or path_parts(fragment_path)
    query = parse_qs(parsed.query or fragment_query)
    if parsed.hostname and parsed.hostname.lower().endswith("163cn.tv"):
        return {
            "platform": "netease_music",
            "kind": "netease_short",
            "id": parts[0] if parts else "",
            "url": url,
        }
    kind_by_path = {
        "song": "netease_song",
        "album": "netease_album",
        "playlist": "netease_playlist",
        "artist": "netease_artist",
    }
    for path_name, kind in kind_by_path.items():
        if path_name in parts:
            resource_id = first_query_value(query, "id")
            if resource_id:
                return {
                    "platform": "netease_music",
                    "kind": kind,
                    "id": resource_id,
                    "url": url,
                }
    return None


def resolve_link_preview(ctx, ref):
    if not ref:
        return None
    resolved = dict(ref)
    if ref["platform"] == "weibo":
        resolved.update(fetch_weibo_status(ctx, ref))
    elif ref["platform"] == "douyin":
        resolved.update(fetch_douyin_title(ctx, ref))
    elif ref["platform"] == "netease_music":
        resolved.update(fetch_netease_metadata(ctx, ref))
    return resolved


def format_link_preview(ref):
    if not ref:
        return ""
    lines = [
        f"{platform_name(ref['platform'])}链接解析",
        f"类型：{LINK_KIND_NAMES.get(ref.get('kind'), '平台链接')}",
    ]
    if ref.get("title"):
        lines.append(f"标题：{ref['title']}")
    if ref.get("author"):
        lines.append(f"作者：{ref['author']}")
    resource_id = str(ref.get("id") or "").strip()
    if resource_id:
        lines.append(f"ID：{resource_id}")
    if ref.get("url"):
        lines.append(f"链接：{ref['url']}")
    if ref.get("message"):
        lines.append(ref["message"])
    return "\n".join(lines)


def fetch_weibo_status(ctx, ref):
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


def fetch_douyin_title(ctx, ref):
    url = str(ref.get("url") or "").strip()
    if not url:
        return {}
    try:
        response = ctx.http_request("GET", url, headers={"User-Agent": "Mozilla/5.0"}, timeout_seconds=12)
    except ActionError as exc:
        return capability_message(exc)
    except Exception:
        return {}
    location = response_header(response, "Location")
    if location and location.startswith("http"):
        parsed = parse_one_url(location)
        if parsed:
            result = {key: value for key, value in parsed.items() if value}
            result.setdefault("message", "短链接已展开。")
            return result
    title = html_title(response)
    return {"title": title[:120]} if title else {}


def fetch_netease_metadata(ctx, ref):
    if ref.get("kind") == "netease_short":
        expanded = expand_short_link(ctx, ref)
        if expanded:
            return expanded
        return {}
    if ref.get("kind") != "netease_song":
        return {}
    song_id = str(ref.get("id") or "").strip()
    if not song_id:
        return {}
    try:
        response = ctx.http_request("GET", f"https://music.163.com/api/song/detail?ids=[{song_id}]", headers={"Referer": "https://music.163.com/"}, timeout_seconds=12)
    except ActionError as exc:
        return capability_message(exc)
    except Exception:
        return {}
    document = json_response(response)
    songs = document.get("songs") if isinstance(document, dict) else []
    if not isinstance(songs, list) or not songs:
        return {}
    song = songs[0] if isinstance(songs[0], dict) else {}
    result = {}
    name = str(song.get("name") or "").strip()
    if name:
        result["title"] = name
    artists = song.get("artists") if isinstance(song.get("artists"), list) else []
    names = [str(item.get("name") or "").strip() for item in artists if isinstance(item, dict) and str(item.get("name") or "").strip()]
    if names:
        result["author"] = "、".join(names)
    return result


def expand_short_link(ctx, ref):
    url = str(ref.get("url") or "").strip()
    if not url:
        return {}
    try:
        response = ctx.http_request("HEAD", url, timeout_seconds=12)
    except ActionError as exc:
        return capability_message(exc)
    except Exception:
        return {}
    location = response_header(response, "Location")
    if not location or not location.startswith("http"):
        return {}
    parsed = parse_one_url(location)
    if not parsed:
        return {}
    result = {key: value for key, value in parsed.items() if value}
    result.setdefault("message", "短链接已展开。")
    return result


def path_parts(path):
    return [part for part in str(path or "").strip("/").split("/") if part]


def first_query_value(query, key):
    values = query.get(key)
    if not values:
        return ""
    return str(values[0] or "").strip()


def json_response(response):
    if not isinstance(response, dict):
        return {}
    try:
        return json.loads(response.get("body_text") or "{}")
    except Exception:
        return {}


def html_title(response):
    if not isinstance(response, dict):
        return ""
    body = response.get("body_text")
    if not isinstance(body, str):
        return ""
    match = re.search(r"<title[^>]*>(.*?)</title>", body, flags=re.IGNORECASE | re.DOTALL)
    return plain_text(match.group(1)) if match else ""


def plain_text(value):
    text = html.unescape(str(value or ""))
    text = re.sub(r"<[^>]+>", "", text)
    return " ".join(text.split()).strip()


def response_header(response, name):
    headers = response.get("headers") if isinstance(response, dict) else {}
    if not isinstance(headers, dict):
        return ""
    for key, value in headers.items():
        if str(key or "").lower() == name.lower():
            return str(value or "").strip()
    return ""


def capability_message(exc):
    if is_http_capability_error(exc):
        return {"message": "链接解析需要订阅中心 manifest 允许对应平台的 http.request host。"}
    return {}
