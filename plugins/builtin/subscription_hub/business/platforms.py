from platforms.bilibili import PLATFORM as BILIBILI_PLATFORM
from platforms.douyin import PLATFORM as DOUYIN_PLATFORM
from platforms.netease_music import PLATFORM as NETEASE_MUSIC_PLATFORM
from platforms.weibo import PLATFORM as WEIBO_PLATFORM


PLATFORM_REGISTRY = {
    item["id"]: item
    for item in (
        BILIBILI_PLATFORM,
        WEIBO_PLATFORM,
        DOUYIN_PLATFORM,
        NETEASE_MUSIC_PLATFORM,
    )
}
PLATFORM_NAMES = {platform_id: item["name"] for platform_id, item in PLATFORM_REGISTRY.items()}
PLATFORM_SUBJECT_LABELS = {platform_id: item["subject_label"] for platform_id, item in PLATFORM_REGISTRY.items()}
SUPPORTED_PLATFORMS = set(PLATFORM_REGISTRY)


def platform_ids():
    return tuple(PLATFORM_REGISTRY)


def normalize_platform(value):
    platform = str(value or "bilibili").strip()
    return platform if platform in SUPPORTED_PLATFORMS else "bilibili"


def platform_metadata(platform):
    return PLATFORM_REGISTRY.get(normalize_platform(platform), BILIBILI_PLATFORM)


def platform_name(platform):
    return platform_metadata(platform).get("name") or PLATFORM_NAMES["bilibili"]


def subject_label(platform):
    return platform_metadata(platform).get("subject_label") or "ID"


def platform_service_names(platform):
    return dict(platform_metadata(platform).get("services") or {})


def platform_service_aliases(platform):
    return dict(platform_metadata(platform).get("service_aliases") or {})


def platform_subscribe_usage(platform):
    return str(platform_metadata(platform).get("subscribe_usage") or "").strip()


def platform_unsubscribe_usage(platform):
    return str(platform_metadata(platform).get("unsubscribe_usage") or "").strip()


def platform_search_usage(platform):
    return str(platform_metadata(platform).get("search_usage") or "").strip()


def subject_text(user, platform=None):
    source = user if isinstance(user, dict) else {}
    platform = normalize_platform(platform or source.get("platform"))
    name = str(source.get("name") or "").strip()
    uid = str(source.get("uid") or "").strip()
    label = subject_label(platform)
    return f"{name}（{label} {uid}）" if name and uid and name != uid else uid or name


def safe_subject_id(value, platform="bilibili"):
    text = str(value or "").strip()
    if normalize_platform(platform) == "bilibili":
        return text if text.isdigit() else ""
    result = []
    for char in text:
        if char.isalnum() or char in "_.-":
            result.append(char)
    return "".join(result).strip("._-")[:96]
