import importlib.util
import pathlib
import unittest
from datetime import date, datetime, timezone
from unittest import mock


PLUGIN_ROOT = pathlib.Path(__file__).resolve().parents[1]
MODULE_PATH = PLUGIN_ROOT / "main.py"
spec = importlib.util.spec_from_file_location("fortune_plugin_main", MODULE_PATH)
fortune = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fortune)


class FortuneLogicTests(unittest.TestCase):
    def test_default_json_loads_and_validates_star_rules(self):
        settings = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")

        self.assertEqual(settings["trigger_commands"], ["我的运势"])
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

        with mock.patch.object(fortune, "ZoneInfo", side_effect=Exception("tzdata missing")):
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

    def test_daily_record_is_stable_for_same_user_and_day(self):
        settings = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")

        first = fortune.build_daily_record(settings, "10001", date(2026, 5, 4))
        second = fortune.build_daily_record(settings, "10001", date(2026, 5, 4))
        next_day = fortune.build_daily_record(settings, "10001", date(2026, 5, 5))

        self.assertEqual(first, second)
        self.assertNotEqual(first["date"], next_day["date"])

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

    def test_actions_file_loads_and_merges_into_defaults(self):
        import json

        with open(PLUGIN_ROOT / "actions.json", "r", encoding="utf-8") as handle:
            raw_actions = json.load(handle)

        defaults = fortune.load_default_settings()

        self.assertGreater(len(defaults["fortunes"]), 0)
        self.assertEqual(defaults["good_actions"], raw_actions["good_actions"])
        self.assertEqual(defaults["bad_actions"], raw_actions["bad_actions"])
        self.assertGreaterEqual(len(defaults["good_actions"]), 2)
        self.assertGreaterEqual(len(defaults["bad_actions"]), 2)

    def test_load_default_settings_falls_back_when_actions_missing(self):
        missing = PLUGIN_ROOT / "_does_not_exist_actions.json"
        self.assertFalse(missing.exists())

        defaults = fortune.load_default_settings(actions_path=str(missing))

        self.assertEqual(defaults["good_actions"], ["整理计划"])
        self.assertEqual(defaults["bad_actions"], ["熬夜"])


if __name__ == "__main__":
    unittest.main()
