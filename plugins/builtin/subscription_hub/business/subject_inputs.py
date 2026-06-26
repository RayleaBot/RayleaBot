import re
from urllib.parse import urlparse, urlunparse

from platforms.douyin import subject_id_from_url as douyin_subject_id_from_url
from platforms.netease_music import subject_id_from_url as netease_music_subject_id_from_url
from platforms.weibo import subject_id_from_url as weibo_subject_id_from_url


URL_PATTERN = re.compile(r"https?://[^\s<>\"]+")

PLATFORM_SUBJECT_EXTRACTORS = {
    "weibo": weibo_subject_id_from_url,
    "douyin": douyin_subject_id_from_url,
    "netease_music": netease_music_subject_id_from_url,
}


def subject_id_from_input(platform, value):
    extractor = PLATFORM_SUBJECT_EXTRACTORS.get(platform)
    if not extractor:
        return ""
    for raw_url in URL_PATTERN.findall(str(value or "")):
        subject_id = extractor(cleanup_url(raw_url))
        if subject_id:
            return subject_id
    return ""


def cleanup_url(value):
    text = str(value or "").strip().rstrip("。），,)")
    parsed = urlparse(text)
    return urlunparse((parsed.scheme, parsed.netloc, parsed.path, "", parsed.query, parsed.fragment))
