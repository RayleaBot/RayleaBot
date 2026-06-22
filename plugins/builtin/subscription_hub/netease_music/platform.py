from urllib.parse import parse_qs

from hub.link_utils import (
    capability_message,
    first_query_value,
    json_response,
    path_parts,
)


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

LINK_KIND_NAMES = {
    "netease_song": "网易云音乐歌曲",
    "netease_album": "网易云音乐专辑",
    "netease_playlist": "网易云音乐歌单",
    "netease_artist": "网易云音乐人",
}


def parse_link(url, parsed):
    host = parsed.hostname.lower() if parsed.hostname else ""
    if not host.endswith("music.163.com"):
        return None
    fragment_path, _, fragment_query = str(parsed.fragment or "").partition("?")
    parts = path_parts(parsed.path) or path_parts(fragment_path)
    query = parse_qs(parsed.query or fragment_query)
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
    from rayleabot.protocol import ActionError

    if ref.get("kind") != "netease_song":
        return {}
    song_id = str(ref.get("id") or "").strip()
    if not song_id:
        return {}
    try:
        response = ctx.http_request(
            "GET",
            f"https://music.163.com/api/song/detail?ids=[{song_id}]",
            headers={"Referer": "https://music.163.com/"},
            timeout_seconds=12,
        )
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
