import importlib.util
import pathlib
import unittest
from datetime import date, datetime, timedelta, timezone
from unittest import mock


PLUGIN_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = PLUGIN_ROOT / "main.py"
spec = importlib.util.spec_from_file_location("fortune_plugin_main", MODULE_PATH)
fortune = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fortune)
fortune_settings = fortune.FortuneSettingsService.__init__.__globals__


class FakeFortuneContext:
    def __init__(self, settings_values=None, storage=None):
        self.event_type = "message.private"
        self.plain_text = "!我的运势"
        self.command_prefixes = ["!"]
        self.payload = {"onebot": {"user_id": 10001, "sender": {"user_id": 10001, "nickname": "Tester"}}}
        self.actor = {"id": "10001", "nickname": "Tester"}
        self.target = {"type": "private", "id": "10001"}
        self.settings_values = settings_values or {}
        self.storage = storage or {}
        self.messages = []
        self.results = []

    def config_read(self, keys):
        return {"values": {key: self.settings_values[key] for key in keys if key in self.settings_values}}

    def storage_get(self, key):
        if key in self.storage:
            return {"exists": True, "value": self.storage[key]}
        return {"exists": False}

    def storage_set(self, key, value):
        self.storage[key] = value
        return {"ok": True}

    def render_image(self, template, data, theme, output, fallback_text):
        self.render_data = data
        return {"image_path": "fortune.png"}

    def send_message(self, segments):
        self.messages.append(segments)

    def send_result(self, result):
        self.results.append(result)

    def logger_write(self, level, message, fields):
        return {"ok": True}


class FortuneLogicTests(unittest.TestCase):
    def test_default_json_loads_and_validates_star_rules(self):
        settings = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")

        self.assertEqual(settings["trigger_commands"], ["我的运势"])
        self.assertEqual(settings["stats_trigger_commands"], ["运势统计"])
        by_name = {item["name"]: item["stars"] for item in settings["fortunes"]}
        self.assertEqual(by_name["大吉"], "★★★★★★★")
        self.assertEqual(by_name["大凶"], "☆☆☆☆☆☆☆")
        self.assertEqual(by_name["吉凶未定"], "???????")
        self.assertIn(by_name["凶"], {"★☆☆☆☆☆☆", "★★☆☆☆☆☆"})

    def test_parse_trigger_uses_configured_tokens_after_global_prefix(self):
        parsed = fortune.parse_trigger("!今日运势", ["!", "/"], ["我的运势", "今日运势"])
        self.assertEqual(parsed["prefix"], "!")
        self.assertEqual(parsed["command"], "今日运势")

        self.assertIsNone(fortune.parse_trigger("今日运势", ["!", "/"], ["今日运势"]))
        self.assertIsNone(fortune.parse_trigger("!未知", ["!", "/"], ["今日运势"]))

    def test_local_date_uses_configured_timezone(self):
        now = datetime(2026, 5, 4, 16, 30, tzinfo=timezone.utc)

        self.assertEqual(fortune.local_date_for_timezone(now, "Asia/Shanghai"), date(2026, 5, 5))
        self.assertEqual(fortune.local_date_for_timezone(now, "UTC"), date(2026, 5, 4))

    def test_local_date_falls_back_when_system_tzdata_is_missing(self):
        now = datetime(2026, 5, 4, 16, 30, tzinfo=timezone.utc)

        with mock.patch.dict(fortune_settings, {"ZoneInfo": mock.Mock(side_effect=Exception("tzdata missing"))}):
            self.assertEqual(fortune.normalize_timezone_name("Asia/Shanghai"), "Asia/Shanghai")
            self.assertEqual(fortune.local_date_for_timezone(now, "Asia/Shanghai"), date(2026, 5, 5))
            self.assertEqual(fortune.local_date_for_timezone(now, "UTC+08:00"), date(2026, 5, 5))
            self.assertEqual(fortune.local_date_for_timezone(now, "+08:00"), date(2026, 5, 5))

    def test_special_date_exact_key_precedes_yearly_key(self):
        settings = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        settings["special_dates"] = [
            {"date": "05-04", "fortune_name": "吉凶未定"},
            {"date": "2026-05-04", "fortune_name": "大吉"},
        ]

        result, special = fortune.fortune_for_date(settings, "10001", date(2026, 5, 4))

        self.assertTrue(special)
        self.assertEqual(result["name"], "大吉")

    def test_special_date_validation_removes_invalid_dates(self):
        settings = {
            "special_dates": [
                {"date": "02-29", "fortune_name": "大吉"},      # Valid (2024 is leap year)
                {"date": "02-31", "fortune_name": "中吉"},      # Invalid (no Feb 31st)
                {"date": "2026-02-29", "fortune_name": "大吉"}, # Invalid (2026 is non-leap year)
                {"date": "05-04", "fortune_name": "吉"},        # Valid
            ]
        }
        validated = fortune.validate_settings(settings)
        dates = [item["date"] for item in validated["special_dates"]]
        self.assertIn("02-29", dates)
        self.assertIn("05-04", dates)
        self.assertNotIn("02-31", dates)
        self.assertNotIn("2026-02-29", dates)

    def test_daily_record_is_stable_for_same_user_and_day(self):
        settings = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")

        first = fortune.build_daily_record(settings, "10001", date(2026, 5, 4))
        second = fortune.build_daily_record(settings, "10001", date(2026, 5, 4))
        next_day = fortune.build_daily_record(settings, "10001", date(2026, 5, 5))

        self.assertEqual(first, second)
        self.assertNotEqual(first["date"], next_day["date"])
        self.assertEqual(first["draw_source_fingerprint"], fortune.draw_source_fingerprint(settings))

    def test_daily_record_matches_numeric_fingerprint(self):
        settings = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        record = fortune.build_daily_record(settings, "10001", date(2026, 5, 4))
        record["draw_source_fingerprint"] = int(record["draw_source_fingerprint"])

        self.assertTrue(fortune.daily_record_matches_settings(record, settings))

    def test_stats_update_counts_first_draw_only_when_called_once(self):
        stats = fortune.empty_stats()
        day = date(2026, 5, 4)

        updated = fortune.update_stats(stats, "大吉", day)

        self.assertEqual(updated["total_days"], 1)
        self.assertEqual(updated["current_streak"], 1)
        self.assertEqual(updated["counts"]["大吉"], 1)
        self.assertEqual(updated["longest_daji_streak"], 1)

    def test_stats_tracks_streaks_and_longest_daji_daxiong(self):
        stats = fortune.empty_stats()
        stats = fortune.update_stats(stats, "大吉", date(2026, 5, 1))
        stats = fortune.update_stats(stats, "大吉", date(2026, 5, 2))
        stats = fortune.update_stats(stats, "吉", date(2026, 5, 3))
        stats = fortune.update_stats(stats, "大凶", date(2026, 5, 4))
        stats = fortune.update_stats(stats, "大凶", date(2026, 5, 5))

        self.assertEqual(stats["total_days"], 5)
        self.assertEqual(stats["current_streak"], 5)
        self.assertEqual(stats["longest_daji_streak"], 2)
        self.assertEqual(stats["longest_daxiong_streak"], 2)
        self.assertEqual(stats["counts"]["大凶"], 2)

    def test_merge_settings_restores_default_values(self):
        defaults = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")

        merged = fortune.merge_settings(defaults, {
            "trigger_commands": ["今日运势"],
            "timezone": "bad/timezone",
            "fortunes": [{"name": "大吉", "stars": "bad", "sign": "x", "explanation": "y"}],
        })

        self.assertEqual(merged["trigger_commands"], ["今日运势"])
        self.assertEqual(merged["timezone"], "Asia/Shanghai")
        self.assertEqual(merged["fortunes"], defaults["fortunes"])

    def test_merge_settings_keeps_custom_fortunes(self):
        defaults = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        custom_fortunes = [{
            "name": "大吉",
            "stars": "★★★★★★★",
            "sign": "自定义签文",
            "explanation": "自定义解签",
        }]

        merged = fortune.merge_settings(defaults, {"fortunes": custom_fortunes})

        self.assertEqual(merged["fortunes"], custom_fortunes)

    def test_merge_settings_keeps_custom_actions(self):
        defaults = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        custom_good = ["自定义宜一", "自定义宜二"]
        custom_bad = ["自定义忌一", "自定义忌二"]

        merged = fortune.merge_settings(defaults, {
            "good_actions": custom_good,
            "bad_actions": custom_bad,
        })

        self.assertEqual(merged["good_actions"], custom_good)
        self.assertEqual(merged["bad_actions"], custom_bad)

    def test_daily_record_detects_draw_source_changes(self):
        defaults = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        alternate = dict(defaults)
        alternate["fortunes"] = [{
            "name": "大吉",
            "stars": "★★★★★★★",
            "sign": "另一套签文",
            "explanation": "另一套解签",
        }]
        day = date(2026, 5, 7)

        old_record = fortune.build_daily_record(alternate, "2022603900", day)
        self.assertFalse(fortune.daily_record_matches_settings(old_record, defaults))

        new_record = fortune.build_daily_record(defaults, "2022603900", day)
        self.assertTrue(fortune.daily_record_matches_settings(new_record, defaults))

    def test_daily_record_detects_action_source_changes(self):
        defaults = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        next_defaults = dict(defaults)
        next_defaults["good_actions"] = ["散步", "读书", "早睡"]
        next_defaults["bad_actions"] = ["熬夜刷屏", "冲动下单", "空腹喝咖啡"]
        day = date(2026, 5, 7)

        old_record = fortune.build_daily_record(defaults, "2022603900", day)

        self.assertFalse(fortune.daily_record_matches_settings(old_record, next_defaults))

    def test_daily_record_excludes_today_good_from_today_bad(self):
        settings = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        overlap = "嫁娶"
        self.assertIn(overlap, settings["good_actions"])
        self.assertIn(overlap, settings["bad_actions"])

        for offset in range(60):
            day = date(2026, 5, 1) + timedelta(days=offset)
            record = fortune.build_daily_record(settings, "10001", day)
            self.assertEqual(set(record["today_good"]) & set(record["today_bad"]), set())

    def test_replace_current_day_fortune_in_stats_does_not_increment_total_days(self):
        stats = fortune.empty_stats()
        day = date(2026, 5, 7)
        stats = fortune.update_stats(stats, "末吉", day)

        replaced = fortune.replace_current_day_fortune_in_stats(stats, "末吉", "大吉")

        self.assertEqual(replaced["total_days"], 1)
        self.assertEqual(replaced["last_date"], day.isoformat())
        self.assertEqual(replaced["counts"]["末吉"], 0)
        self.assertEqual(replaced["counts"]["大吉"], 1)
        self.assertEqual(replaced["current_daji_streak"], 1)
        self.assertEqual(replaced["longest_daji_streak"], 1)

    def test_replace_current_day_fortune_in_stats_adjusts_daxiong_streak(self):
        stats = fortune.empty_stats()
        day = date(2026, 5, 7)
        stats = fortune.update_stats(stats, "大凶", day)

        replaced = fortune.replace_current_day_fortune_in_stats(stats, "大凶", "吉")

        self.assertEqual(replaced["total_days"], 1)
        self.assertEqual(replaced["counts"]["大凶"], 0)
        self.assertEqual(replaced["counts"]["吉"], 1)
        self.assertEqual(replaced["current_daxiong_streak"], 0)

    def test_message_reuses_cached_record_from_current_draw_source(self):
        plugin = fortune.FortunePlugin()
        defaults = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        day = date(2026, 5, 7)
        existing_record = fortune.build_daily_record(defaults, "10001", day)
        existing_stats = fortune.update_stats(fortune.empty_stats(), existing_record["fortune"]["name"], day)
        storage = {
            "daily:10001:2026-05-07": existing_record,
            "stats:10001": existing_stats,
        }
        ctx = FakeFortuneContext(storage=storage)

        with mock.patch.object(fortune, "local_date_for_timezone", return_value=day):
            plugin.handle_message(ctx)

        next_record = storage["daily:10001:2026-05-07"]
        next_stats = storage["stats:10001"]
        self.assertEqual(ctx.results[-1], {"handled": True})
        self.assertEqual(next_record, existing_record)
        self.assertEqual(next_record["draw_source_fingerprint"], fortune.draw_source_fingerprint(defaults))
        self.assertEqual(next_stats["total_days"], 1)
        self.assertEqual(next_stats["counts"][next_record["fortune"]["name"]], 1)

    def test_default_settings_include_actions(self):
        import json

        with open(PLUGIN_ROOT / "fortunes.json", "r", encoding="utf-8") as handle:
            raw_defaults = json.load(handle)

        defaults = fortune.load_default_settings()

        self.assertGreater(len(defaults["fortunes"]), 0)
        self.assertEqual(defaults["good_actions"], raw_defaults["good_actions"])
        self.assertEqual(defaults["bad_actions"], raw_defaults["bad_actions"])
        self.assertGreaterEqual(len(defaults["good_actions"]), 2)
        self.assertGreaterEqual(len(defaults["bad_actions"]), 2)


if __name__ == "__main__":
    unittest.main()
