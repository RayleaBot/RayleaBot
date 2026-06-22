import html
import json
import re

from .http_utils import is_http_capability_error


def path_parts(path):
    return [part for part in str(path or "").strip("/").split("/") if part]


def first_query_value(query, key):
    values = query.get(key)
    if not values:
        return ""
    return str(values[0] or "").strip()


def json_response(response):
    if not isinstance(response, dict):
        return {}
    try:
        return json.loads(response.get("body_text") or "{}")
    except Exception:
        return {}


def html_title(response):
    if not isinstance(response, dict):
        return ""
    body = response.get("body_text")
    if not isinstance(body, str):
        return ""
    match = re.search(r"<title[^>]*>(.*?)</title>", body, flags=re.IGNORECASE | re.DOTALL)
    return plain_text(match.group(1)) if match else ""


def plain_text(value):
    text = html.unescape(str(value or ""))
    text = re.sub(r"<[^>]+>", "", text)
    return " ".join(text.split()).strip()


def capability_message(exc):
    if is_http_capability_error(exc):
        return {"message": "链接解析需要订阅中心 manifest 允许对应平台的 http.request host。"}
    return {}
