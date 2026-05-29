import html
from html.parser import HTMLParser


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
    subscriber_cards = subscriber_card_data(subscribers)
    original = render_original(update.get("original"))
    author = update.get("author") or {"name": subscription.get("name") or subscription.get("uid")}
    title = limit_title(update.get("title") or "订阅更新")
    summary = limit_text(update.get("summary") or "", 420)
    summary_html = limit_html(update.get("summary_html") or "", 420)
    service = service_label(update.get("service"))
    category = update.get("category") or service
    images = list(update.get("images") or [])[:9]
    media_images = images_with_duration(images, update.get("duration_text"), service)
    media = media_data(media_images, service)
    return {
        "title": title,
        "headline": title,
        "content_text": summary,
        "content_html": summary_html,
        "subtitle": f"Bilibili · {service}",
        "source_label": f"Bilibili · {category}",
        "platform": "Bilibili",
        "service": service,
        "category": category,
        "author": author,
        "author_uid_text": uid_text(author.get("uid") or subscription.get("uid")),
        "summary": summary,
        "summary_html": summary_html,
        "images": images,
        "image_count": media["count"],
        "media_grid_class": media["grid_class"],
        "media_items": media["items"],
        "duration_text": str(update.get("duration_text") or "").strip(),
        "url": update.get("url") or "",
        "pub_ts": int(update.get("pub_ts") or 0),
        "created_at": update.get("created_at") or "",
        "live_event": update.get("live_event") or "",
        "live_status": update.get("live_status"),
        "status_label": update.get("status_label") or "",
        "live_started_at": update.get("live_started_at") or "",
        "live_detected_at": update.get("live_detected_at") or "",
        "original": original,
        "subscription": {
            "uid": subscription.get("uid"),
            "name": subscription.get("name") or subscription.get("uid"),
        },
        "subscribers": subscribers,
        "subscriber_cards": subscriber_cards,
        "subscriber_text": format_subscribers(subscriber_cards),
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
        text = str(item.get("display_name") or item.get("nickname") or item.get("id") or "").strip()
        if text:
            names.append(text)
    return "、".join(names)


def subscriber_card_data(subscribers):
    cards = []
    for item in subscribers or []:
        if not isinstance(item, dict):
            continue
        subscriber_id = str(item.get("id") or "").strip()
        if not subscriber_id:
            continue
        nickname = str(item.get("nickname") or subscriber_id).strip() or subscriber_id
        group_nickname = str(item.get("group_nickname") or item.get("card") or "").strip()
        display_name = group_nickname or nickname
        role = str(item.get("role") or "").strip()
        role_label = str(item.get("role_label") or role_label_for(role)).strip()
        title = str(item.get("title") or "").strip()
        avatar_url = str(item.get("avatar_url") or qq_avatar_url(subscriber_id)).strip()
        cards.append({
            "id": subscriber_id,
            "nickname": nickname,
            "group_nickname": group_nickname,
            "display_name": display_name,
            "title": title,
            "role": role,
            "role_label": role_label,
            "avatar_url": avatar_url,
            "uid_text": subscriber_id,
        })
    return cards


def role_label_for(role):
    return {
        "super_admin": "超级管理员",
        "owner": "群主",
        "admin": "管理员",
        "member": "群员",
    }.get(role, "")


def qq_avatar_url(subscriber_id):
    if subscriber_id.isdigit():
        return f"https://q1.qlogo.cn/g?b=qq&nk={subscriber_id}&s=100"
    return ""


def uid_text(value):
    text = str(value or "").strip()
    return f"UID {text}" if text else ""


def render_original(original):
    if not isinstance(original, dict):
        return None
    service = service_label(original.get("service"))
    images = list(original.get("images") or [])[:6]
    media_images = images_with_duration(images, original.get("duration_text"), service)
    media = media_data(media_images, service)
    return {
        "title": limit_title(original.get("title") or "原动态"),
        "service": service,
        "category": original.get("category") or service,
        "author": original.get("author") or {},
        "author_uid_text": uid_text((original.get("author") or {}).get("uid")) if isinstance(original.get("author"), dict) else "",
        "summary": limit_text(original.get("summary") or "", 260),
        "summary_html": limit_html(original.get("summary_html") or "", 260),
        "images": images[:3],
        "image_count": media["count"],
        "media_grid_class": media["grid_class"],
        "media_items": media["items"],
        "duration_text": str(original.get("duration_text") or "").strip(),
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


def limit_html(value, limit):
    text = str(value or "").strip()
    if html_visible_length(text) <= limit:
        return text
    return truncate_html(text, limit)


class VisibleTextParser(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.length = 0

    def handle_data(self, data):
        self.length += len(data)


def html_visible_length(value):
    parser = VisibleTextParser()
    parser.feed(str(value or ""))
    parser.close()
    return parser.length


class HTMLTruncator(HTMLParser):
    VOID_TAGS = {"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track", "wbr"}

    def __init__(self, limit):
        super().__init__(convert_charrefs=False)
        self.limit = limit
        self.length = 0
        self.parts = []
        self.open_tags = []
        self.truncated = False

    def handle_starttag(self, tag, attrs):
        if self.truncated:
            return
        self.parts.append(self.format_start_tag(tag, attrs))
        if tag.lower() not in self.VOID_TAGS:
            self.open_tags.append(tag)

    def handle_startendtag(self, tag, attrs):
        if not self.truncated:
            self.parts.append(self.format_start_tag(tag, attrs, closed=True))

    def handle_endtag(self, tag):
        if self.truncated:
            return
        self.parts.append(f"</{tag}>")
        for index in range(len(self.open_tags) - 1, -1, -1):
            if self.open_tags[index].lower() == tag.lower():
                del self.open_tags[index:]
                break

    def handle_data(self, data):
        if self.truncated or not data:
            return
        remaining = self.limit - self.length
        if len(data) <= remaining:
            self.parts.append(html.escape(data, quote=False))
            self.length += len(data)
            return
        self.parts.append(html.escape(data[:remaining].rstrip(), quote=False))
        self.parts.append("...")
        self.truncated = True

    def handle_entityref(self, name):
        self.handle_data(html.unescape(f"&{name};"))

    def handle_charref(self, name):
        self.handle_data(html.unescape(f"&#{name};"))

    def format_start_tag(self, tag, attrs, closed=False):
        attr_text = "".join(
            f' {name}="{html.escape(str(value), quote=True)}"' if value is not None else f" {name}"
            for name, value in attrs
        )
        return f"<{tag}{attr_text}{' /' if closed else ''}>"

    def result(self):
        for tag in reversed(self.open_tags):
            self.parts.append(f"</{tag}>")
        return "".join(self.parts)


def truncate_html(value, limit):
    parser = HTMLTruncator(limit)
    parser.feed(str(value or ""))
    parser.close()
    return parser.result()


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


def images_with_duration(images, duration_text, service):
    result = []
    for image in list(images or [])[:9]:
        if isinstance(image, dict):
            result.append(dict(image))
        else:
            result.append(image)
    duration = str(duration_text or "").strip()
    if duration and service_label(service) == "视频" and result and isinstance(result[0], dict):
        result[0]["duration_text"] = duration
    return result


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
    duration_text = str(image.get("duration_text") or "").strip()
    classes = ["media-item"]
    if service in {"视频", "直播", "文章"}:
        classes.append("media-item--wide")
    if is_long:
        classes.append("media-item--long")
    if is_gif:
        classes.append("media-item--gif")
    if duration_text:
        classes.append("media-item--video")
    return {
        "url": url,
        "class": " ".join(classes),
        "label": " · ".join(labels),
        "duration_text": duration_text,
        "width": width,
        "height": height,
    }


def safe_int(value):
    try:
        number = int(value or 0)
    except (TypeError, ValueError):
        return 0
    return number if number > 0 else 0
