from .platforms import normalize_platform


PLATFORM_SERVICE_NAMES = {
    "bilibili": {
        "all": "全部",
        "live": "直播",
        "video": "视频",
        "image_text": "图文",
        "article": "文章",
        "repost": "转发",
    },
    "weibo": {
        "all": "全部",
        "post": "微博",
        "image": "图片",
        "video": "视频",
        "repost": "转发",
    },
    "douyin": {
        "all": "全部",
        "video": "视频",
        "image_text": "图文",
        "live": "直播",
    },
    "netease_music": {
        "all": "全部",
        "song": "歌曲",
        "album": "专辑",
        "playlist": "歌单",
        "artist": "音乐人",
    },
}

SERVICE_NAMES = PLATFORM_SERVICE_NAMES["bilibili"]

PLATFORM_SERVICE_ALIASES = {
    "bilibili": {
        "全部": "all",
        "全量": "all",
        "所有": "all",
        "直播": "live",
        "视频": "video",
        "图文": "image_text",
        "动态": "image_text",
        "文章": "article",
        "专栏": "article",
        "转发": "repost",
        "live": "live",
        "video": "video",
        "image_text": "image_text",
        "article": "article",
        "repost": "repost",
        "all": "all",
    },
    "weibo": {
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
    "douyin": {
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
    "netease_music": {
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


def service_names_for(platform):
    return PLATFORM_SERVICE_NAMES.get(normalize_platform(platform), SERVICE_NAMES)


def service_order_for(platform):
    return list(service_names_for(platform).keys())


def service_types_for(platform):
    return [service for service in service_order_for(platform) if service != "all"]


def service_enabled(subscription, service):
    platform = normalize_platform(subscription.get("platform"))
    services = set(normalize_services(subscription.get("services"), platform))
    return "all" in services or service in services


def normalize_service_token(value, platform="bilibili"):
    aliases = PLATFORM_SERVICE_ALIASES.get(normalize_platform(platform), PLATFORM_SERVICE_ALIASES["bilibili"])
    return aliases.get(str(value or "").strip().lower()) or aliases.get(str(value or "").strip())


def subscription_id_for(platform, uid, target_type, target_id):
    return f"{platform}-{uid}-{target_type}-{target_id}"


def normalize_services(value, platform="bilibili"):
    names = service_names_for(platform)
    source = value if isinstance(value, list) else ["all"]
    result = []
    for item in source:
        service = str(item or "").strip()
        if service in names and service not in result:
            result.append(service)
    if not result or "all" in result:
        return ["all"]
    service_types = service_types_for(platform)
    return ["all"] if all(service in result for service in service_types) else result


def merge_services(existing, incoming, platform="bilibili"):
    names = service_names_for(platform)
    current = [service for service in existing or [] if service in names]
    if "all" in current or "all" in incoming:
        return ["all"]
    result = []
    for service in current + incoming:
        if service in names and service not in result:
            result.append(service)
    return result or ["all"]


def remove_services(existing, removing, platform="bilibili"):
    names = service_names_for(platform)
    current = [service for service in existing or ["all"] if service in names]
    if "all" in removing:
        return []
    if "all" in current:
        current = service_types_for(platform)
    return [service for service in current if service not in removing]


def services_text(services, platform="bilibili"):
    names = service_names_for(platform)
    values = normalize_services(services, platform)
    return "、".join(names.get(service, service) for service in values)


def digits(value):
    text = str(value or "").strip()
    return text if text.isdigit() else ""
