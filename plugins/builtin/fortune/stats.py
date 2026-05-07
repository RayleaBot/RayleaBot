from datetime import date, timedelta

from settings import COUNTED_FORTUNES


class FortuneStatsService:
    def empty(self):
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

    def normalize(self, value):
        stats = self.empty()
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

    def update(self, stats, fortune_name, local_day):
        next_stats = self.normalize(stats)
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

    def replace_current_day_fortune(self, stats, previous_fortune_name, next_fortune_name):
        next_stats = self.normalize(stats)
        if previous_fortune_name == next_fortune_name:
            return next_stats
        if previous_fortune_name in next_stats["counts"]:
            next_stats["counts"][previous_fortune_name] = max(0, next_stats["counts"][previous_fortune_name] - 1)
        if next_fortune_name in next_stats["counts"]:
            next_stats["counts"][next_fortune_name] += 1
        if previous_fortune_name == "大吉" and next_fortune_name != "大吉":
            next_stats["current_daji_streak"] = max(0, next_stats["current_daji_streak"] - 1)
        elif previous_fortune_name != "大吉" and next_fortune_name == "大吉":
            next_stats["current_daji_streak"] += 1
            next_stats["longest_daji_streak"] = max(next_stats["longest_daji_streak"], next_stats["current_daji_streak"])
        if previous_fortune_name == "大凶" and next_fortune_name != "大凶":
            next_stats["current_daxiong_streak"] = max(0, next_stats["current_daxiong_streak"] - 1)
        elif previous_fortune_name != "大凶" and next_fortune_name == "大凶":
            next_stats["current_daxiong_streak"] += 1
            next_stats["longest_daxiong_streak"] = max(next_stats["longest_daxiong_streak"], next_stats["current_daxiong_streak"])
        return next_stats

    def summary(self, stats):
        normalized = self.normalize(stats)
        items = []
        for name in COUNTED_FORTUNES:
            items.append({"label": f"累计{name}", "value": f"{normalized['counts'].get(name, 0)} 次"})
        items.append({"label": "最长连续大吉", "value": f"{normalized['longest_daji_streak']} 天"})
        items.append({"label": "最长连续大凶", "value": f"{normalized['longest_daxiong_streak']} 天"})
        return items


def non_negative_int(value):
    try:
        number = int(value)
    except (TypeError, ValueError):
        return 0
    return max(0, number)


def parse_iso_date(value):
    try:
        return date.fromisoformat(str(value))
    except ValueError:
        return None


_DEFAULT_SERVICE = FortuneStatsService()


def empty_stats():
    return _DEFAULT_SERVICE.empty()


def normalize_stats(value):
    return _DEFAULT_SERVICE.normalize(value)


def update_stats(stats, fortune_name, local_day):
    return _DEFAULT_SERVICE.update(stats, fortune_name, local_day)


def replace_current_day_fortune_in_stats(stats, previous_fortune_name, next_fortune_name):
    return _DEFAULT_SERVICE.replace_current_day_fortune(stats, previous_fortune_name, next_fortune_name)


def build_stats_summary(stats):
    return _DEFAULT_SERVICE.summary(stats)
