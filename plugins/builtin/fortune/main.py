#!/usr/bin/env python3
"""Built-in daily fortune plugin for RayleaBot."""

import copy
import hashlib
import json
import os
import random
import re
import sys
import threading
from datetime import date, datetime, timedelta, timezone
from zoneinfo import ZoneInfo

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "..", "sdk", "python"))

from rayleabot import RayleaBotPlugin, event_handler


PLUGIN_DIR = os.path.dirname(__file__)
DEFAULT_SETTINGS_PATH = os.path.join(PLUGIN_DIR, "fortunes.json")
DEFAULT_ACTIONS_PATH = os.path.join(PLUGIN_DIR, "actions.json")
ACTIONS_KEYS = ("good_actions", "bad_actions")
DEFAULT_TIMEZONE = "Asia/Shanghai"
DEFAULT_TRIGGER_COMMANDS = ["我的运势"]
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
    "timezone",
    "fortunes",
    "special_dates",
    "good_actions",
    "bad_actions",
]


class SettingsValidationError(ValueError):
    pass


def load_default_settings(path=DEFAULT_SETTINGS_PATH, actions_path=DEFAULT_ACTIONS_PATH):
    with open(path, "r", encoding="utf-8") as handle:
        document = json.load(handle)
    apply_actions_overlay(document, actions_path)
    return validate_settings(document, require_usable=True)


def apply_actions_overlay(document, actions_path):
    if not actions_path:
        return
    try:
        with open(actions_path, "r", encoding="utf-8") as handle:
            overlay = json.load(handle)
    except FileNotFoundError:
        return
    if not isinstance(overlay, dict):
        return
    for key in ACTIONS_KEYS:
        if key in overlay:
            document[key] = overlay[key]


def validate_settings(raw_settings, require_usable=False):
    source = raw_settings if isinstance(raw_settings, dict) else {}
    settings = {
        "trigger_commands": normalize_string_list(source.get("trigger_commands"), DEFAULT_TRIGGER_COMMANDS),
        "timezone": normalize_timezone_name(source.get("timezone")),
        "fortunes": normalize_fortunes(source.get("fortunes")),
        "special_dates": normalize_special_dates(source.get("special_dates")),
        "good_actions": normalize_string_list(source.get("good_actions"), ["整理计划"]),
        "bad_actions": normalize_string_list(source.get("bad_actions"), ["熬夜"]),
    }

    if require_usable and not settings["fortunes"]:
        raise SettingsValidationError("默认运势库没有可用条目")
    if not settings["fortunes"]:
        settings["fortunes"] = normalize_fortunes(load_default_settings()["fortunes"])
    return settings


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
    return bool(re.fullmatch(r"\d{4}-\d{2}-\d{2}", value) or re.fullmatch(r"\d{2}-\d{2}", value))


def merge_settings(default_settings, stored_values):
    merged = copy.deepcopy(default_settings)
    if isinstance(stored_values, dict):
        for key in SETTINGS_KEYS:
            if key in stored_values:
                merged[key] = stored_values[key]
    normalized = validate_settings(merged)
    if not normalized["fortunes"]:
        normalized["fortunes"] = copy.deepcopy(default_settings["fortunes"])
    return normalized


def settings_issue_messages(stored_values):
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


def parse_trigger(plain_text, prefixes, trigger_commands):
    text = (plain_text or "").strip()
    if not text:
        return None
    sorted_prefixes = sorted([prefix for prefix in prefixes if prefix], key=len, reverse=True)
    for prefix in sorted_prefixes:
        if not text.startswith(prefix):
            continue
        rest = text[len(prefix):].strip()
        for command in trigger_commands:
            if rest == command or rest.startswith(command + " "):
                return {
                    "prefix": prefix,
                    "command": command,
                    "args": rest[len(command):].strip(),
                }
    return None


def local_date_for_timezone(now, timezone_name):
    if now is None:
        now = datetime.now(timezone.utc)
    if now.tzinfo is None:
        now = now.replace(tzinfo=timezone.utc)
    return now.astimezone(resolve_timezone_info(timezone_name)).date()


def stable_seed(*parts):
    joined = "|".join(str(part) for part in parts)
    return int(hashlib.sha256(joined.encode("utf-8")).hexdigest(), 16)


def choose_from_list(values, seed, count):
    items = [str(item).strip() for item in values if str(item).strip()]
    if not items:
        return []
    rng = random.Random(seed)
    if len(items) <= count:
        shuffled = list(items)
        rng.shuffle(shuffled)
        return shuffled
    return rng.sample(items, count)


def fortune_for_date(settings, user_id, local_day):
    special = match_special_date(settings, local_day)
    if special is not None:
        return copy.deepcopy(special), True

    fortunes = [fortune for fortune in settings["fortunes"] if fortune["name"] != SPECIAL_FORTUNE]
    if not fortunes:
        fortunes = settings["fortunes"]
    index = stable_seed(user_id, local_day.isoformat(), "fortune") % len(fortunes)
    return copy.deepcopy(fortunes[index]), False


def match_special_date(settings, local_day):
    exact_key = local_day.isoformat()
    yearly_key = local_day.strftime("%m-%d")
    by_key = {item["date"]: item for item in settings.get("special_dates", [])}
    for key in (exact_key, yearly_key):
        item = by_key.get(key)
        if not item:
            continue
        fortune = item.get("fortune")
        if isinstance(fortune, dict):
            normalized = normalize_fortune(fortune)
            if normalized is not None:
                return normalized
        fortune_name = str(item.get("fortune_name") or "").strip()
        matched = find_fortune_by_name(settings["fortunes"], fortune_name)
        if matched is not None:
            return matched
    return None


def find_fortune_by_name(fortunes, name):
    for fortune in fortunes:
        if fortune.get("name") == name:
            return copy.deepcopy(fortune)
    return None


def build_daily_record(settings, user_id, local_day):
    fortune, special = fortune_for_date(settings, user_id, local_day)
    return {
        "date": local_day.isoformat(),
        "fortune": fortune,
        "today_good": choose_from_list(settings["good_actions"], stable_seed(user_id, local_day.isoformat(), "good"), 2),
        "today_bad": choose_from_list(settings["bad_actions"], stable_seed(user_id, local_day.isoformat(), "bad"), 2),
        "special": special,
    }


def empty_stats():
    return {
        "total_days": 0,
        "current_streak": 0,
        "last_date": "",
        "counts": {name: 0 for name in COUNTED_FORTUNES},
        "current_daji_streak": 0,
        "longest_daji_streak": 0,
        "current_daxiong_streak": 0,
        "longest_daxiong_streak": 0,
    }


def normalize_stats(value):
    stats = empty_stats()
    if not isinstance(value, dict):
        return stats
    stats["total_days"] = non_negative_int(value.get("total_days"))
    stats["current_streak"] = non_negative_int(value.get("current_streak"))
    stats["last_date"] = str(value.get("last_date") or "")
    counts = value.get("counts")
    if isinstance(counts, dict):
        for name in COUNTED_FORTUNES:
            stats["counts"][name] = non_negative_int(counts.get(name))
    for key in ("current_daji_streak", "longest_daji_streak", "current_daxiong_streak", "longest_daxiong_streak"):
        stats[key] = non_negative_int(value.get(key))
    return stats


def non_negative_int(value):
    try:
        number = int(value)
    except (TypeError, ValueError):
        return 0
    return max(0, number)


def update_stats(stats, fortune_name, local_day):
    next_stats = normalize_stats(stats)
    day_text = local_day.isoformat()
    previous = parse_iso_date(next_stats["last_date"])
    if previous == local_day - timedelta(days=1):
        next_stats["current_streak"] += 1
    elif previous == local_day:
        pass
    else:
        next_stats["current_streak"] = 1

    if previous != local_day:
        next_stats["total_days"] += 1
        next_stats["last_date"] = day_text

    if fortune_name in next_stats["counts"]:
        next_stats["counts"][fortune_name] += 1

    if fortune_name == "大吉":
        next_stats["current_daji_streak"] += 1
    else:
        next_stats["current_daji_streak"] = 0
    next_stats["longest_daji_streak"] = max(next_stats["longest_daji_streak"], next_stats["current_daji_streak"])

    if fortune_name == "大凶":
        next_stats["current_daxiong_streak"] += 1
    else:
        next_stats["current_daxiong_streak"] = 0
    next_stats["longest_daxiong_streak"] = max(next_stats["longest_daxiong_streak"], next_stats["current_daxiong_streak"])
    return next_stats


def parse_iso_date(value):
    try:
        return date.fromisoformat(str(value))
    except ValueError:
        return None


def build_stats_summary(stats):
    normalized = normalize_stats(stats)
    items = []
    for name in COUNTED_FORTUNES:
        items.append({"label": f"累计{name}", "value": f"{normalized['counts'].get(name, 0)} 次"})
    items.append({"label": "最长连续大吉", "value": f"{normalized['longest_daji_streak']} 天"})
    items.append({"label": "最长连续大凶", "value": f"{normalized['longest_daxiong_streak']} 天"})
    return items


def storage_value(result, fallback=None):
    if isinstance(result, dict):
        if result.get("exists") and "value" in result:
            return result.get("value")
        if "value" in result:
            return result.get("value")
    return fallback


def user_identity_from_context(ctx):
    payload = ctx.payload
    onebot = payload.get("onebot") if isinstance(payload.get("onebot"), dict) else {}
    sender = onebot.get("sender") if isinstance(onebot.get("sender"), dict) else {}
    actor = ctx.actor
    user_id = str(onebot.get("user_id") or sender.get("user_id") or actor.get("id") or "").strip()
    nickname = str(sender.get("nickname") or actor.get("nickname") or user_id or "访客").strip()
    group_nickname = str(sender.get("card") or "").strip()
    title = str(sender.get("title") or actor.get("title") or "").strip()
    avatar_url = ""
    if user_id and user_id.isdigit():
        avatar_url = f"https://q1.qlogo.cn/g?b=qq&nk={user_id}&s=100"
    return {
        "id": user_id or "unknown",
        "nickname": nickname or user_id or "访客",
        "group_nickname": group_nickname,
        "title": title,
        "avatar_url": avatar_url,
    }


def group_identity_from_context(ctx):
    target = ctx.target
    if target.get("type") != "group":
        return {}
    name = str(target.get("name") or target.get("id") or "").strip()
    return {"name": name} if name else {}


def build_render_data(ctx, settings, record, stats, repeated):
    local_date = record["date"]
    timezone_name = settings["timezone"]
    return {
        "title": "今日运势",
        "subtitle": local_date,
        "repeat_notice": "今日运势已经抽取过，以下为当日结果。" if repeated else "",
        "user": user_identity_from_context(ctx),
        "group": group_identity_from_context(ctx),
        "fortune": record["fortune"],
        "today_good": record.get("today_good") or [],
        "today_bad": record.get("today_bad") or [],
        "streak": {
            "current": normalize_stats(stats)["current_streak"],
            "total": normalize_stats(stats)["total_days"],
        },
        "stats": build_stats_summary(stats),
    }


def build_fallback_text(render_data):
    fortune = render_data["fortune"]
    streak = render_data["streak"]
    lines = [
        render_data["title"],
        render_data.get("repeat_notice") or "",
        f"运势：{fortune['name']}",
        f"星级：{fortune['stars']}",
        f"签文：{fortune['sign']}",
        f"解签：{fortune['explanation']}",
        f"今日宜：{'、'.join(render_data.get('today_good') or [])}",
        f"今日忌：{'、'.join(render_data.get('today_bad') or [])}",
        f"你已经连续查看运势 {streak['current']} 天。累计查看运势 {streak['total']} 天。",
    ]
    return "\n".join(line for line in lines if line)


class FortunePlugin(RayleaBotPlugin):
    def __init__(self):
        super().__init__()
        self.subscribe("message.group", "message.private", "config.changed")
        self._settings_lock = threading.Lock()
        self._default_settings = load_default_settings()
        self._settings = copy.deepcopy(self._default_settings)
        self._settings_loaded = False

    def current_settings(self):
        with self._settings_lock:
            return copy.deepcopy(self._settings)

    def set_current_settings(self, settings):
        with self._settings_lock:
            self._settings = copy.deepcopy(settings)
            self._settings_loaded = True

    def settings_loaded(self):
        with self._settings_lock:
            return self._settings_loaded

    def load_settings(self, ctx, force=False):
        if self.settings_loaded() and not force:
            return self.current_settings()

        try:
            response = ctx.config_read(SETTINGS_KEYS)
            values = response.get("values", {}) if isinstance(response, dict) else {}
            settings = merge_settings(self._default_settings, values)
            for message in settings_issue_messages(values):
                self.try_log(ctx, "warn", message)
        except Exception as exc:
            settings = copy.deepcopy(self._default_settings)
            self.try_log(ctx, "warn", "运势设置读取失败，使用默认设置", {"error": str(exc)})

        self.set_current_settings(settings)
        return settings

    def try_log(self, ctx, level, message, fields=None):
        try:
            ctx.logger_write(level, message, fields or {})
        except Exception:
            pass

    @event_handler("config.changed")
    def handle_config_changed(self, ctx):
        self.load_settings(ctx, force=True)
        ctx.send_result({"handled": True})

    @event_handler()
    def handle_message(self, ctx):
        if ctx.event_type not in {"message.group", "message.private"}:
            ctx.send_result({"handled": False})
            return

        settings = self.load_settings(ctx)
        trigger = parse_trigger(ctx.plain_text, ctx.command_prefixes, settings["trigger_commands"])
        if trigger is None:
            ctx.send_result({"handled": False})
            return

        user = user_identity_from_context(ctx)
        user_id = user["id"]
        local_day = local_date_for_timezone(None, settings["timezone"])
        daily_key = f"daily:{user_id}:{local_day.isoformat()}"
        stats_key = f"stats:{user_id}"

        record_result = ctx.storage_get(daily_key)
        record = storage_value(record_result)
        repeated = isinstance(record, dict)

        stats = normalize_stats(storage_value(ctx.storage_get(stats_key), empty_stats()))
        if not repeated:
            record = build_daily_record(settings, user_id, local_day)
            stats = update_stats(stats, record["fortune"]["name"], local_day)
            ctx.storage_set(daily_key, record)
            ctx.storage_set(stats_key, stats)

        render_data = build_render_data(ctx, settings, record, stats, repeated)
        self.send_fortune_image(ctx, render_data)
        ctx.send_result({"handled": True})

    def send_fortune_image(self, ctx, render_data):
        fallback_text = build_fallback_text(render_data)
        result = ctx.render_image(
            "fortune.card",
            render_data,
            theme="default",
            output="png",
            fallback_text=fallback_text,
        )
        image_path = str(result.get("image_path") or "").strip()
        if image_path:
            ctx.send_message([{
                "type": "image",
                "data": {"file": image_path},
            }])
            return
        self.try_log(ctx, "warn", "运势图片生成结果缺少图片路径")


if __name__ == "__main__":
    FortunePlugin().run()
