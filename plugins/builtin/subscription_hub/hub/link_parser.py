import re
from urllib.parse import urlparse, urlunparse

from douyin import LINK_KIND_NAMES as DOUYIN_LINK_KIND_NAMES
from douyin import parse_link as parse_douyin_link
from douyin import resolve_link_preview as resolve_douyin_link_preview
from netease_music import LINK_KIND_NAMES as NETEASE_MUSIC_LINK_KIND_NAMES
from netease_music import parse_link as parse_netease_music_link
from netease_music import resolve_link_preview as resolve_netease_music_link_preview
from .platforms import platform_name
from weibo import LINK_KIND_NAMES as WEIBO_LINK_KIND_NAMES
from weibo import parse_link as parse_weibo_link
from weibo import resolve_link_preview as resolve_weibo_link_preview


URL_PATTERN = re.compile(r"https?://[^\s<>\"]+")

LINK_KIND_NAMES = {
    **WEIBO_LINK_KIND_NAMES,
    **DOUYIN_LINK_KIND_NAMES,
    **NETEASE_MUSIC_LINK_KIND_NAMES,
}

PLATFORM_LINK_PARSERS = (
    parse_weibo_link,
    parse_douyin_link,
    parse_netease_music_link,
)

PLATFORM_LINK_RESOLVERS = {
    "weibo": resolve_weibo_link_preview,
    "douyin": resolve_douyin_link_preview,
    "netease_music": resolve_netease_music_link_preview,
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
    for parser in PLATFORM_LINK_PARSERS:
        ref = parser(url, parsed)
        if ref:
            return ref
    return None


def resolve_link_preview(ctx, ref):
    if not ref:
        return None
    resolved = dict(ref)
    resolver = PLATFORM_LINK_RESOLVERS.get(ref.get("platform"))
    if resolver:
        resolved.update(resolver(ctx, ref))
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
