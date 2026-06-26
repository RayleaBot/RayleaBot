import base64

from platforms.bilibili import bilibili_document_error, parse_json_response


def preview_response_document(response, label):
    failure_label = f"Bilibili {label}预览失败"
    response_failure = bilibili_response_failure(response, failure_label)
    if response_failure:
        return response_failure
    document = parse_json_response(response)
    error = bilibili_document_error(document)
    if error:
        message = error.get("message") or "Bilibili 响应读取失败。"
        if error.get("kind") == "not_found":
            message = f"没有找到这个 Bilibili {label}。"
        return f"{failure_label}：{sentence_text(message)}{response_details_text(response)}"
    return document


def bilibili_response_failure(response, label):
    status_code = response.get("status_code") if isinstance(response, dict) else None
    if not isinstance(status_code, int) or status_code < 200 or status_code >= 300:
        return f"{label}：{response_details_text(response)}"
    if not parse_json_response(response):
        return f"{label}：Bilibili 返回内容不是 JSON。{response_details_text(response)}"
    return None


def response_details_text(response):
    status_code = response.get("status_code") if isinstance(response, dict) else None
    body_excerpt = response_body_excerpt(response)
    return f"HTTP {http_status_text(status_code)}{response_excerpt_suffix(body_excerpt)}"


def http_status_text(status_code):
    return str(status_code) if isinstance(status_code, int) else "未知"


def sentence_text(text):
    text = str(text or "").strip()
    if not text:
        return ""
    return text if text.endswith(("。", "！", "？", ".", "!", "?")) else text + "。"


def response_body_excerpt(response, limit=600):
    if not isinstance(response, dict):
        return ""
    body = response.get("body_text")
    if isinstance(body, str):
        text = body
    else:
        body_base64 = response.get("body_base64")
        if not isinstance(body_base64, str) or not body_base64.strip():
            return ""
        try:
            raw = base64.b64decode(body_base64, validate=True)
        except Exception:
            return "[binary response]"
        text = raw.decode("utf-8", errors="replace")
    text = " ".join(str(text or "").split())
    if len(text) <= limit:
        return text
    return text[:limit].rstrip() + "..."


def response_excerpt_suffix(excerpt):
    return f"。响应：{excerpt}" if excerpt else "。"


def is_http_capability_error(exc):
    code = str(getattr(exc, "code", "") or "").lower()
    message = str(exc or "").lower()
    details = str(getattr(exc, "details", "") or "").lower()
    combined = " ".join([code, message, details])
    return (
        "capability" in combined
        or "capability_parameters" in combined
        or "http_hosts" in combined
        or "scope" in combined
    )
