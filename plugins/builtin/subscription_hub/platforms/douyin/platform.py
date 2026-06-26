from platforms.url_tools import hostname_matches, parsed_url, path_parts


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

def subject_id_from_url(url):
    parsed = parsed_url(url)
    host = parsed.hostname.lower() if parsed.hostname else ""
    if not hostname_matches(host, "douyin.com", "iesdouyin.com", "amemv.com"):
        return ""
    parts = path_parts(parsed.path)
    for marker in ("video", "note"):
        if marker in parts:
            index = parts.index(marker)
            if len(parts) > index + 1:
                return parts[index + 1]
    return ""
