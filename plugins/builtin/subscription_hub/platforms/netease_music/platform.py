from platforms.url_tools import first_query_value, hostname_matches, parsed_url, path_parts, query_values


PLATFORM = {
    "id": "netease_music",
    "name": "网易云音乐",
    "subject_label": "ID",
    "subscribe_usage": "用法：/订阅网易云音乐推送 [歌曲|专辑|歌单|音乐人] ID或主页标识；类型可选，不填表示全部类型。",
    "unsubscribe_usage": "用法：/取消网易云音乐推送 [歌曲|专辑|歌单|音乐人] ID或主页标识；类型可选，不填表示全部类型。",
    "services": {
        "all": "全部",
        "song": "歌曲",
        "album": "专辑",
        "playlist": "歌单",
        "artist": "音乐人",
    },
    "service_aliases": {
        "全部": "all",
        "全量": "all",
        "所有": "all",
        "歌曲": "song",
        "音乐": "song",
        "单曲": "song",
        "专辑": "album",
        "歌单": "playlist",
        "音乐人": "artist",
        "歌手": "artist",
        "song": "song",
        "album": "album",
        "playlist": "playlist",
        "artist": "artist",
        "all": "all",
    },
}

def subject_id_from_url(url):
    parsed = parsed_url(url)
    host = parsed.hostname.lower() if parsed.hostname else ""
    if not hostname_matches(host, "music.163.com"):
        return ""
    fragment_path, _, fragment_query = str(parsed.fragment or "").partition("?")
    parts = path_parts(parsed.path) or path_parts(fragment_path)
    query = query_values(parsed.query or fragment_query)
    for path_name in ("song", "album", "playlist", "artist"):
        if path_name in parts:
            resource_id = first_query_value(query, "id")
            if resource_id:
                return resource_id
    return ""
