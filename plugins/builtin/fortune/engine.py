import copy
import hashlib
import json
import random
from datetime import datetime, timezone

from settings import (
    SPECIAL_FORTUNE,
    normalize_fortune,
    normalize_fortunes,
    normalize_special_dates,
    normalize_string_list,
    resolve_timezone_info,
)


def stable_seed(*parts):
    joined = "|".join(str(part) for part in parts)
    return int(hashlib.sha256(joined.encode("utf-8")).hexdigest(), 16)


def stable_fingerprint(value):
    return stable_seed(json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")))


def draw_source_fingerprint_value(settings):
    return str(stable_fingerprint({
        "fortunes": normalize_fortunes(settings.get("fortunes")),
        "special_dates": normalize_special_dates(settings.get("special_dates")),
        "good_actions": normalize_string_list(settings.get("good_actions"), ["整理计划"]),
        "bad_actions": normalize_string_list(settings.get("bad_actions"), ["熬夜"]),
    }))


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


class DailyFortuneEngine:
    def __init__(self, settings):
        self.settings = settings
        self.fortunes = normalize_fortunes(settings.get("fortunes"))
        self.special_dates = normalize_special_dates(settings.get("special_dates"))
        self.drawable_fortunes = [fortune for fortune in self.fortunes if fortune["name"] != SPECIAL_FORTUNE] or self.fortunes
        self.special_dates_by_key = {item["date"]: item for item in self.special_dates}
        self.draw_source_fingerprint = self.fingerprint_settings(settings)

    @classmethod
    def fingerprint_settings(cls, settings):
        return draw_source_fingerprint_value(settings)

    def build_daily_record(self, user_id, local_day):
        fortune, special = self.fortune_for_date(user_id, local_day)
        today_good = choose_from_list(
            self.settings["good_actions"],
            stable_seed(user_id, local_day.isoformat(), "good"),
            2,
        )
        today_bad_pool = [item for item in self.settings["bad_actions"] if item not in today_good]
        today_bad = choose_from_list(
            today_bad_pool,
            stable_seed(user_id, local_day.isoformat(), "bad"),
            2,
        )
        return {
            "date": local_day.isoformat(),
            "draw_source_fingerprint": self.draw_source_fingerprint,
            "fortune": fortune,
            "today_good": today_good,
            "today_bad": today_bad,
            "special": special,
        }

    def fortune_for_date(self, user_id, local_day):
        special = self.match_special_date(local_day)
        if special is not None:
            return copy.deepcopy(special), True

        index = stable_seed(user_id, local_day.isoformat(), "fortune") % len(self.drawable_fortunes)
        return copy.deepcopy(self.drawable_fortunes[index]), False

    def match_special_date(self, local_day):
        exact_key = local_day.isoformat()
        yearly_key = local_day.strftime("%m-%d")
        for key in (exact_key, yearly_key):
            item = self.special_dates_by_key.get(key)
            if not item:
                continue
            fortune = item.get("fortune")
            if isinstance(fortune, dict):
                normalized = normalize_fortune(fortune)
                if normalized is not None:
                    return normalized
            matched = self.find_fortune_by_name(str(item.get("fortune_name") or "").strip())
            if matched is not None:
                return matched
        return None

    def find_fortune_by_name(self, name):
        for fortune in self.fortunes:
            if fortune.get("name") == name:
                return copy.deepcopy(fortune)
        return None

    def record_matches_settings(self, record):
        if not isinstance(record, dict):
            return False
        return str(record.get("draw_source_fingerprint")) == self.draw_source_fingerprint


def build_daily_record(settings, user_id, local_day):
    return DailyFortuneEngine(settings).build_daily_record(user_id, local_day)


def fortune_for_date(settings, user_id, local_day):
    return DailyFortuneEngine(settings).fortune_for_date(user_id, local_day)


def match_special_date(settings, local_day):
    return DailyFortuneEngine(settings).match_special_date(local_day)


def find_fortune_by_name(fortunes, name):
    settings = {"fortunes": fortunes, "special_dates": [], "good_actions": [], "bad_actions": []}
    return DailyFortuneEngine(settings).find_fortune_by_name(name)


def draw_source_fingerprint(settings):
    return DailyFortuneEngine.fingerprint_settings(settings)


def daily_record_matches_settings(record, settings):
    return DailyFortuneEngine(settings).record_matches_settings(record)
