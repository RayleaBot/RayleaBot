import copy

from .services import SERVICE_NAMES, service_enabled


def normalize_bilibili_event_payload(payload):
    if not isinstance(payload, dict):
        return None
    update = copy.deepcopy(payload)
    update_id = str(update.get("id") or "").strip()
    uid = str(update.get("uid") or "").strip()
    service = str(update.get("service") or "").strip()
    if not update_id or not uid or service not in SERVICE_NAMES or service == "all":
        return None

    update["id"] = update_id
    update["uid"] = uid
    update["service"] = service
    update.setdefault("category", SERVICE_NAMES.get(service, service))
    author = update.get("author") if isinstance(update.get("author"), dict) else {}
    if not str(author.get("uid") or "").strip():
        author["uid"] = uid
    if not str(author.get("name") or "").strip():
        author["name"] = uid
    update["author"] = author
    update["images"] = normalize_bilibili_images(update.get("images"))
    topic = normalize_bilibili_topic(update.get("topic"))
    if topic:
        update["topic"] = topic
    elif "topic" in update:
        update.pop("topic", None)
    original = normalize_bilibili_original(update.get("original"))
    if original:
        update["original"] = original
    elif "original" in update:
        update.pop("original", None)
    return update


def normalize_bilibili_original(value):
    if not isinstance(value, dict):
        return None
    original = copy.deepcopy(value)
    update_id = str(original.get("id") or "").strip()
    service = str(original.get("service") or "").strip()
    url = str(original.get("url") or "").strip()
    if not update_id or service not in SERVICE_NAMES or service == "all" or not url:
        return None
    original["id"] = update_id
    original["service"] = service
    original.setdefault("category", SERVICE_NAMES.get(service, service))
    author = original.get("author") if isinstance(original.get("author"), dict) else {}
    if not str(author.get("uid") or "").strip():
        return None
    if not str(author.get("name") or "").strip():
        author["name"] = str(author.get("uid") or "").strip()
    original["author"] = author
    original["images"] = normalize_bilibili_images(original.get("images"))
    topic = normalize_bilibili_topic(original.get("topic"))
    if topic:
        original["topic"] = topic
    elif "topic" in original:
        original.pop("topic", None)
    return original


def normalize_bilibili_topic(value):
    if not isinstance(value, dict):
        return None
    name = str(value.get("name") or "").strip().strip("# \t\r\n")
    if not name:
        return None
    topic = {"name": name}
    try:
        topic_id = int(value.get("id") or 0)
    except (TypeError, ValueError):
        topic_id = 0
    if topic_id > 0:
        topic["id"] = topic_id
    jump_url = str(value.get("jump_url") or "").strip()
    if jump_url:
        topic["jump_url"] = jump_url
    return topic


def normalize_bilibili_images(value):
    images = value if isinstance(value, list) else []
    return [item for item in images if isinstance(item, dict) and str(item.get("url") or "").strip()]


def subscription_matches_event(subscription, update):
    return (
        subscription.get("platform") == "bilibili"
        and str(subscription.get("uid") or "").strip() == str(update.get("uid") or "").strip()
        and service_enabled(subscription, update.get("service"))
    )
