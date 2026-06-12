SERVICE_ALIASES = {
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
}

SERVICE_NAMES = {
    "all": "全部",
    "live": "直播",
    "video": "视频",
    "image_text": "图文",
    "article": "文章",
    "repost": "转发",
}


def service_enabled(subscription, service):
    services = set(subscription.get("services") or ["all"])
    return "all" in services or service in services


def normalize_service_token(value):
    return SERVICE_ALIASES.get(str(value or "").strip().lower()) or SERVICE_ALIASES.get(str(value or "").strip())


def subscription_id_for(platform, uid, target_type, target_id):
    return f"{platform}-{uid}-{target_type}-{target_id}"


def merge_services(existing, incoming):
    current = [service for service in existing or [] if service in SERVICE_NAMES]
    if "all" in current or "all" in incoming:
        return ["all"]
    result = []
    for service in current + incoming:
        if service in SERVICE_NAMES and service not in result:
            result.append(service)
    return result or ["all"]


def remove_services(existing, removing):
    current = [service for service in existing or ["all"] if service in SERVICE_NAMES]
    if "all" in removing:
        return []
    if "all" in current:
        current = ["live", "video", "image_text", "article", "repost"]
    return [service for service in current if service not in removing]


def services_text(services):
    values = services or ["all"]
    return "、".join(SERVICE_NAMES.get(service, service) for service in values)


def digits(value):
    text = str(value or "").strip()
    return text if text.isdigit() else ""
