SERVICE_LABELS = {
    "live": "直播",
    "video": "视频",
    "image_text": "图文",
    "article": "文章",
    "repost": "转发",
    "all": "全部",
}


def service_label(service):
    return SERVICE_LABELS.get(service, service or "内容")


def build_render_data(subscription, update):
    subscribers = subscription.get("subscribers") or []
    original = render_original(update.get("original"))
    author = update.get("author") or {"name": subscription.get("name") or subscription.get("uid")}
    title = limit_title(update.get("title") or "订阅更新")
    summary = limit_text(update.get("summary") or "", 420)
    service = service_label(update.get("service"))
    category = update.get("category") or service
    images = list(update.get("images") or [])[:9]
    media = media_data(images, service)
    return {
        "title": title,
        "headline": title,
        "content_text": summary,
        "subtitle": f"Bilibili · {service}",
        "source_label": f"Bilibili · {category}",
        "platform": "Bilibili",
        "service": service,
        "category": category,
        "author": author,
        "summary": summary,
        "images": images,
        "image_count": media["count"],
        "media_grid_class": media["grid_class"],
        "media_items": media["items"],
        "url": update.get("url") or "",
        "pub_ts": int(update.get("pub_ts") or 0),
        "created_at": update.get("created_at") or "",
        "original": original,
        "subscription": {
            "uid": subscription.get("uid"),
            "name": subscription.get("name") or subscription.get("uid"),
        },
        "subscribers": subscribers,
        "subscriber_text": format_subscribers(subscribers),
    }


def build_fallback_text(data):
    lines = [
        data.get("source_label") or data.get("subtitle") or "Bilibili 订阅更新",
        data.get("headline") or data.get("title") or "",
        data.get("content_text") or data.get("summary") or "",
        original_fallback(data.get("original")),
        f"订阅人：{data.get('subscriber_text')}" if data.get("subscriber_text") else "",
        data.get("url") or "",
    ]
    return "\n".join(line for line in lines if line)


def format_subscribers(subscribers):
    names = []
    for item in subscribers or []:
        if not isinstance(item, dict):
            continue
        text = str(item.get("nickname") or item.get("id") or "").strip()
        if text:
            names.append(text)
    return "、".join(names)


def render_original(original):
    if not isinstance(original, dict):
        return None
    service = service_label(original.get("service"))
    images = list(original.get("images") or [])[:6]
    media = media_data(images, service)
    return {
        "title": limit_title(original.get("title") or "原动态"),
        "service": service,
        "category": original.get("category") or service,
        "author": original.get("author") or {},
        "summary": limit_text(original.get("summary") or "", 260),
        "images": images[:3],
        "image_count": media["count"],
        "media_grid_class": media["grid_class"],
        "media_items": media["items"],
        "url": original.get("url") or "",
        "created_at": original.get("created_at") or "",
    }


def original_fallback(original):
    if not isinstance(original, dict):
        return ""
    parts = ["原动态", original.get("title") or "", original.get("summary") or ""]
    return "\n".join(part for part in parts if part)


def limit_text(value, limit):
    text = "\n".join(" ".join(line.split()) for line in str(value or "").splitlines()).strip()
    if len(text) <= limit:
        return text
    return text[:limit].rstrip() + "..."


def limit_title(value):
    return limit_text(value, 72)


def media_data(images, service):
    items = []
    for image in images:
        item = media_item(image, service)
        if item:
            items.append(item)
    return {
        "count": len(items),
        "grid_class": media_grid_class(len(items)),
        "items": items,
    }


def media_grid_class(count):
    if count <= 0:
        return ""
    if count == 1:
        return "media-grid--single"
    if count in (2, 4):
        return "media-grid--double"
    return "media-grid--triple"


def media_item(image, service):
    if not isinstance(image, dict):
        return None
    url = str(image.get("url") or "").strip()
    if not url:
        return None
    width = safe_int(image.get("width"))
    height = safe_int(image.get("height"))
    is_gif = url.split("?", 1)[0].lower().endswith(".gif")
    is_long = width > 0 and height > width * 2
    labels = []
    if is_gif:
        labels.append("动图")
    if is_long:
        labels.append("长图")
    classes = ["media-item"]
    if service in {"视频", "直播", "文章"}:
        classes.append("media-item--wide")
    if is_long:
        classes.append("media-item--long")
    if is_gif:
        classes.append("media-item--gif")
    return {
        "url": url,
        "class": " ".join(classes),
        "label": " · ".join(labels),
        "width": width,
        "height": height,
    }


def safe_int(value):
    try:
        number = int(value or 0)
    except (TypeError, ValueError):
        return 0
    return number if number > 0 else 0
