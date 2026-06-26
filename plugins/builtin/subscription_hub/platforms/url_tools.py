from urllib.parse import parse_qs, urlparse


def parsed_url(value):
    return urlparse(str(value or "").strip())


def path_parts(path):
    return [part for part in str(path or "").strip("/").split("/") if part]


def hostname_matches(host, *suffixes):
    normalized_host = str(host or "").strip().lower().rstrip(".")
    if not normalized_host:
        return False
    for suffix in suffixes:
        normalized_suffix = str(suffix or "").strip().lower().lstrip(".")
        if normalized_host == normalized_suffix or normalized_host.endswith(f".{normalized_suffix}"):
            return True
    return False


def query_values(query):
    return parse_qs(str(query or ""))


def first_query_value(query, key):
    values = query.get(key)
    if not values:
        return ""
    return str(values[0] or "").strip()
