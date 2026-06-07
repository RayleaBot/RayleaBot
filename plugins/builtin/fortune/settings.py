import copy
import json
import re
from datetime import timedelta, timezone
from zoneinfo import ZoneInfo


DEFAULT_TIMEZONE = "Asia/Shanghai"
DEFAULT_TRIGGER_COMMANDS = ["我的运势"]
DEFAULT_STATS_TRIGGER_COMMANDS = ["运势统计"]
FORTUNE_ORDER = ["大吉", "吉", "中吉", "小吉", "末吉", "凶", "大凶"]
SPECIAL_FORTUNE = "吉凶未定"
FIXED_TIMEZONES = {
    "UTC": timezone.utc,
    "Etc/UTC": timezone.utc,
    "Asia/Shanghai": timezone(timedelta(hours=8), DEFAULT_TIMEZONE),
    "PRC": timezone(timedelta(hours=8), "PRC"),
}
COUNTED_FORTUNES = FORTUNE_ORDER + [SPECIAL_FORTUNE]
EXPECTED_STARS = {
    "大吉": "★★★★★★★",
    "吉": "★★★★★★☆",
    "中吉": "★★★★★☆☆",
    "小吉": "★★★★☆☆☆",
    "末吉": "★★★☆☆☆☆",
    "大凶": "☆☆☆☆☆☆☆",
    SPECIAL_FORTUNE: "???????",
}
FIERCE_STARS = {"★☆☆☆☆☆☆", "★★☆☆☆☆☆"}
SETTINGS_KEYS = [
    "trigger_commands",
    "stats_trigger_commands",
    "timezone",
    "fortunes",
    "special_dates",
    "good_actions",
    "bad_actions",
]


class SettingsValidationError(ValueError):
    pass


def stable_json(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


class FortuneSettingsService:
    def __init__(self, default_settings_path):
        self.default_settings_path = default_settings_path

    def load_default_settings(self):
        with open(self.default_settings_path, "r", encoding="utf-8") as handle:
            document = json.load(handle)
        return self.validate(document, require_usable=True)

    def validate(self, raw_settings, require_usable=False):
        source = raw_settings if isinstance(raw_settings, dict) else {}
        settings = {
            "trigger_commands": normalize_string_list(source.get("trigger_commands"), DEFAULT_TRIGGER_COMMANDS),
            "stats_trigger_commands": normalize_string_list(source.get("stats_trigger_commands"), DEFAULT_STATS_TRIGGER_COMMANDS),
            "timezone": normalize_timezone_name(source.get("timezone")),
            "fortunes": normalize_fortunes(source.get("fortunes")),
            "special_dates": normalize_special_dates(source.get("special_dates")),
            "good_actions": normalize_string_list(source.get("good_actions"), ["整理计划"]),
            "bad_actions": normalize_string_list(source.get("bad_actions"), ["熬夜"]),
        }

        if require_usable and not settings["fortunes"]:
            raise SettingsValidationError("默认运势库没有可用条目")
        if not settings["fortunes"]:
            settings["fortunes"] = normalize_fortunes(self.load_default_settings()["fortunes"])
        return settings

    def merge(self, default_settings, stored_values):
        merged = copy.deepcopy(default_settings)
        if isinstance(stored_values, dict):
            for key in SETTINGS_KEYS:
                if key not in stored_values:
                    continue
                merged[key] = stored_values[key]
        normalized = self.validate(merged)
        if not normalized["fortunes"]:
            normalized["fortunes"] = copy.deepcopy(default_settings["fortunes"])
        return normalized

    def issue_messages(self, stored_values):
        if not isinstance(stored_values, dict):
            return []

        messages = []
        if "timezone" in stored_values:
            raw_timezone = str(stored_values.get("timezone") or "").strip()
            if raw_timezone and normalize_timezone_name(raw_timezone) != raw_timezone:
                messages.append("运势时区无效，使用默认时区")

        fortunes = stored_values.get("fortunes")
        if isinstance(fortunes, list):
            valid_count = len(normalize_fortunes(fortunes))
            if len(fortunes) > 0 and valid_count == 0:
                messages.append("运势覆盖没有可用条目，使用默认运势库")
            elif valid_count < len(fortunes):
                messages.append("部分运势条目无效，已跳过")

        special_dates = stored_values.get("special_dates")
        if isinstance(special_dates, list) and len(normalize_special_dates(special_dates)) < len(special_dates):
            messages.append("部分特殊日期无效，已跳过")

        return messages


def normalize_string_list(value, fallback):
    source = value if isinstance(value, list) else fallback
    items = []
    seen = set()
    for item in source:
        text = str(item).strip()
        if not text or text in seen:
            continue
        seen.add(text)
        items.append(text)
    return items or list(fallback)


def normalize_timezone_name(value):
    name = str(value or "").strip() or DEFAULT_TIMEZONE
    if timezone_name_supported(name):
        return name
    return DEFAULT_TIMEZONE


def timezone_name_supported(name):
    timezone_name = str(name or "").strip() or DEFAULT_TIMEZONE
    try:
        ZoneInfo(timezone_name)
        return True
    except Exception:
        return fixed_timezone_info(timezone_name) is not None


def resolve_timezone_info(name):
    timezone_name = str(name or "").strip() or DEFAULT_TIMEZONE
    try:
        return ZoneInfo(timezone_name)
    except Exception:
        fixed = fixed_timezone_info(timezone_name)
        if fixed is not None:
            return fixed
        if timezone_name != DEFAULT_TIMEZONE:
            return resolve_timezone_info(DEFAULT_TIMEZONE)
        return timezone.utc


def fixed_timezone_info(name):
    timezone_name = str(name or "").strip()
    if timezone_name in FIXED_TIMEZONES:
        return FIXED_TIMEZONES[timezone_name]

    match = re.fullmatch(r"(?:UTC)?([+-])(\d{1,2})(?::?(\d{2}))?", timezone_name, re.IGNORECASE)
    if not match:
        return None

    sign, hour_text, minute_text = match.groups()
    hours = int(hour_text)
    minutes = int(minute_text or "0")
    if hours > 14 or minutes > 59 or (hours == 14 and minutes != 0):
        return None

    offset = timedelta(hours=hours, minutes=minutes)
    if sign == "-":
        offset = -offset
    return timezone(offset, timezone_name)


def normalize_fortunes(value):
    if not isinstance(value, list):
        return []

    fortunes = []
    for item in value:
        fortune = normalize_fortune(item)
        if fortune is not None:
            fortunes.append(fortune)
    return fortunes


def normalize_fortune(item):
    if not isinstance(item, dict):
        return None
    name = str(item.get("name") or "").strip()
    stars = str(item.get("stars") or "").strip()
    sign = str(item.get("sign") or "").strip()
    explanation = str(item.get("explanation") or "").strip()
    if not name or not stars or not sign or not explanation:
        return None
    if not valid_stars_for_fortune(name, stars):
        return None
    return {
        "name": name,
        "stars": stars,
        "sign": sign,
        "explanation": explanation,
    }


def valid_stars_for_fortune(name, stars):
    if len(stars) != 7:
        return False
    if name == "凶":
        return stars in FIERCE_STARS
    expected = EXPECTED_STARS.get(name)
    if expected is None:
        return False
    return stars == expected


def normalize_special_dates(value):
    if not isinstance(value, list):
        return []

    items = []
    for item in value:
        if not isinstance(item, dict):
            continue
        raw_date = str(item.get("date") or "").strip()
        if not is_special_date_key(raw_date):
            continue
        fortune_name = str(item.get("fortune_name") or item.get("fortune") or "").strip()
        fortune = normalize_fortune(item.get("fortune")) if isinstance(item.get("fortune"), dict) else None
        if not fortune_name and fortune is None:
            continue
        items.append({
            "date": raw_date,
            "fortune_name": fortune_name or fortune["name"],
            **({"fortune": fortune} if fortune is not None else {}),
        })
    return items


def is_special_date_key(value):
    if not isinstance(value, str):
        return False
    if re.fullmatch(r"\d{4}-\d{2}-\d{2}", value):
        parts = value.split("-")
        try:
            y = int(parts[0])
            m = int(parts[1])
            d = int(parts[2])
            from datetime import date
            date(y, m, d)
            return True
        except ValueError:
            return False
    elif re.fullmatch(r"\d{2}-\d{2}", value):
        parts = value.split("-")
        try:
            m = int(parts[0])
            d = int(parts[1])
            from datetime import date
            # Use 2024 (a leap year) to allow 02-29, but block 02-30, 02-31 etc.
            date(2024, m, d)
            return True
        except ValueError:
            return False
    return False
