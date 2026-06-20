PLATFORM_NAMES = {
    "bilibili": "Bilibili",
    "weibo": "微博",
    "douyin": "抖音",
    "netease_music": "网易云音乐",
}

PLATFORM_SUBJECT_LABELS = {
    "bilibili": "UID",
    "weibo": "UID",
    "douyin": "抖音号",
    "netease_music": "ID",
}

SUPPORTED_PLATFORMS = set(PLATFORM_NAMES)


def normalize_platform(value):
    platform = str(value or "bilibili").strip()
    return platform if platform in SUPPORTED_PLATFORMS else "bilibili"


def platform_name(platform):
    return PLATFORM_NAMES.get(normalize_platform(platform), PLATFORM_NAMES["bilibili"])


def subject_label(platform):
    return PLATFORM_SUBJECT_LABELS.get(normalize_platform(platform), "ID")


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
