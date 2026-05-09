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
    return {
        "title": update.get("title") or "订阅更新",
        "subtitle": f"Bilibili · {service_label(update.get('service'))}",
        "platform": "Bilibili",
        "service": service_label(update.get("service")),
        "author": update.get("author") or {"name": subscription.get("name") or subscription.get("uid")},
        "summary": update.get("summary") or "",
        "url": update.get("url") or "",
        "created_at": update.get("created_at") or "",
        "subscription": {
            "uid": subscription.get("uid"),
            "name": subscription.get("name") or subscription.get("uid"),
        },
        "subscribers": subscribers,
        "subscriber_text": format_subscribers(subscribers),
    }


def build_fallback_text(data):
    lines = [
        data.get("subtitle") or "Bilibili 订阅更新",
        data.get("title") or "",
        data.get("summary") or "",
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
