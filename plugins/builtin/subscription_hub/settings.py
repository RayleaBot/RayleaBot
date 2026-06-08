import copy


SETTINGS_KEYS = [
    "enabled",
    "subscriptions",
]

SERVICE_KINDS = {"live", "video", "image_text", "article", "repost", "all"}
DEFAULT_SETTINGS = {
    "enabled": True,
    "subscriptions": [],
}


def merge_settings(default_settings, stored_values):
    merged = copy.deepcopy(default_settings or DEFAULT_SETTINGS)
    if isinstance(stored_values, dict):
        for key in SETTINGS_KEYS:
            if key in stored_values:
                merged[key] = stored_values[key]
    return normalize_settings(merged)


def normalize_settings(raw):
    source = raw if isinstance(raw, dict) else {}
    return {
        "enabled": bool(source.get("enabled", True)),
        "subscriptions": normalize_subscriptions(source.get("subscriptions")),
    }


def normalize_subscriptions(value):
    items = []
    seen = set()
    source = value if isinstance(value, list) else []
    for item in source:
        if not isinstance(item, dict):
            continue
        uid = digits(item.get("uid"))
        target_type = str(item.get("target_type") or "").strip()
        target_id = str(item.get("target_id") or "").strip()
        if not uid or target_type not in {"group", "private"} or not target_id:
            continue
        subscription_id = safe_id(item.get("id")) or f"bilibili-{uid}-{target_type}-{target_id}"
        dedupe = subscription_id
        if dedupe in seen:
            continue
        seen.add(dedupe)
        services = normalize_services(item.get("services"))
        subscription = {
            "id": subscription_id,
            "platform": "bilibili",
            "uid": uid,
            "name": str(item.get("name") or uid).strip() or uid,
            "target_type": target_type,
            "target_id": target_id,
            "services": services,
            "subscribers": normalize_subscribers(item.get("subscribers")),
            "enabled": bool(item.get("enabled", True)),
        }
        for key in ("avatar_url", "target_name"):
            text = str(item.get(key) or "").strip()
            if text:
                subscription[key] = text
        items.append(subscription)
    return items


def normalize_services(value):
    source = value if isinstance(value, list) else ["all"]
    result = []
    seen = set()
    for item in source:
        service = str(item or "").strip()
        if service not in SERVICE_KINDS or service in seen:
            continue
        seen.add(service)
        result.append(service)
    return result or ["all"]


def normalize_subscribers(value):
    items = []
    source = value if isinstance(value, list) else []
    for item in source:
        if isinstance(item, dict):
            subscriber_id = str(item.get("id") or "").strip()
            nickname = str(item.get("nickname") or subscriber_id).strip()
            if subscriber_id:
                subscriber = {"id": subscriber_id, "nickname": nickname or subscriber_id}
                for key in ("group_nickname", "title", "role", "role_label", "avatar_url"):
                    text = str(item.get(key) or "").strip()
                    if text:
                        subscriber[key] = text
                items.append(subscriber)
        else:
            text = str(item or "").strip()
            if text:
                items.append({"id": text, "nickname": text})
    return items


def safe_id(value):
    text = str(value or "").strip().lower()
    result = []
    for char in text:
        if char.isalnum() or char in "_.-":
            result.append(char)
    normalized = "".join(result).strip("._-")
    return normalized[:96]


def digits(value):
    text = str(value or "").strip()
    return text if text.isdigit() else ""
