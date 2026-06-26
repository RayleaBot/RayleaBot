from platforms.url_tools import first_query_value, hostname_matches, parsed_url, path_parts, query_values


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

def subject_id_from_url(url):
    parsed = parsed_url(url)
    host = parsed.hostname.lower() if parsed.hostname else ""
    if not hostname_matches(host, "weibo.com", "weibo.cn"):
        return ""
    parts = path_parts(parsed.path)
    query = query_values(parsed.query)
    status_id = first_query_value(query, "id")
    if not status_id and len(parts) >= 2 and parts[0] in {"status", "detail"}:
        status_id = parts[1]
    if not status_id and len(parts) >= 2:
        status_id = parts[1]
    if not status_id:
        return ""
    return status_id
