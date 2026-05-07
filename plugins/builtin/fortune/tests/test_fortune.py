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
    def legacy_default_settings(self):
        return {
            "trigger_commands": ["我的运势"],
            "timezone": "Asia/Shanghai",
            "fortunes": [
                {
                    "name": "大吉",
                    "stars": "★★★★★★★",
                    "sign": "云开见月，万事顺遂，贵人相助，所愿可期",
                    "explanation": "适合推进重要事项，保持节奏会有不错结果。",
                },
                {
                    "name": "吉",
                    "stars": "★★★★★★☆",
                    "sign": "春风入户，行路有光，小有波折，终见坦途",
                    "explanation": "整体顺利，细节处多确认即可稳住进展。",
                },
                {
                    "name": "中吉",
                    "stars": "★★★★★☆☆",
                    "sign": "稳步前行，守正得安，急则易乱，缓则有成",
                    "explanation": "适合按计划完成手头事务，不宜临时改变方向。",
                },
                {
                    "name": "小吉",
                    "stars": "★★★★☆☆☆",
                    "sign": "小事可成，大事宜缓，静观其变，自有回音",
                    "explanation": "保持耐心，先处理确定性高的事情。",
                },
                {
                    "name": "末吉",
                    "stars": "★★★☆☆☆☆",
                    "sign": "运势微明，仍需勤勉，少言多做，可避烦忧",
                    "explanation": "结果需要积累，不适合依赖临场发挥。",
                },
                {
                    "name": "凶",
                    "stars": "★★☆☆☆☆☆",
                    "sign": "遇事犹疑，难望成事，大刀阔斧，始可有成",
                    "explanation": "做事犹豫、不果断，很难做成功；变得果断勇敢了，才有希望。",
                },
                {
                    "name": "大凶",
                    "stars": "☆☆☆☆☆☆☆",
                    "sign": "乌云压境，诸事宜慎，闭门修整，静待转机",
                    "explanation": "适合降低预期，先处理风险和遗留问题。",
                },
                {
                    "name": "吉凶未定",
                    "stars": "???????",
                    "sign": "风云未定，机缘未明，一念之间，吉凶自分",
                    "explanation": "今天的结果更依赖选择本身，保持清醒判断。",
                },
            ],
            "special_dates": [],
            "good_actions": ["整理计划", "主动沟通", "早睡早起", "处理积压事项", "学习新知识", "备份资料"],
            "bad_actions": ["拖延决定", "冲动消费", "熬夜", "重复争辩", "临时改约", "忽略细节"],
        }

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
        self.assertEqual(first["draw_source_fingerprint"], fortune.draw_source_fingerprint(settings))

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

    def test_merge_settings_ignores_seeded_legacy_default_fortunes(self):
        defaults = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        legacy = self.legacy_default_settings()

        merged = fortune.merge_settings(defaults, {
            "fortunes": legacy["fortunes"],
            "trigger_commands": ["今日运势"],
        })

        self.assertEqual(merged["trigger_commands"], ["今日运势"])
        self.assertEqual(merged["fortunes"], defaults["fortunes"])
        self.assertGreater(len(merged["fortunes"]), len(legacy["fortunes"]))

    def test_merge_settings_ignores_seeded_legacy_default_actions(self):
        defaults = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        legacy = self.legacy_default_settings()
        next_defaults = dict(defaults)
        next_defaults["good_actions"] = ["散步", "读书", "早睡"]
        next_defaults["bad_actions"] = ["熬夜刷屏", "冲动下单", "空腹喝咖啡"]

        merged = fortune.merge_settings(next_defaults, {
            "good_actions": legacy["good_actions"],
            "bad_actions": legacy["bad_actions"],
        })

        self.assertEqual(merged["good_actions"], next_defaults["good_actions"])
        self.assertEqual(merged["bad_actions"], next_defaults["bad_actions"])

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
        legacy = fortune.validate_settings(self.legacy_default_settings(), require_usable=True)
        day = date(2026, 5, 7)

        old_record = fortune.build_daily_record(legacy, "2022603900", day)
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

    def test_message_refreshes_cached_record_from_legacy_draw_source(self):
        plugin = fortune.FortunePlugin()
        defaults = fortune.load_default_settings(PLUGIN_ROOT / "fortunes.json")
        legacy = fortune.validate_settings(self.legacy_default_settings(), require_usable=True)
        day = date(2026, 5, 7)
        old_record = fortune.build_daily_record(legacy, "10001", day)
        old_stats = fortune.update_stats(fortune.empty_stats(), old_record["fortune"]["name"], day)
        storage = {
            "daily:10001:2026-05-07": old_record,
            "stats:10001": old_stats,
        }
        ctx = FakeFortuneContext(storage=storage)

        with mock.patch.object(fortune, "local_date_for_timezone", return_value=day):
            plugin.handle_message(ctx)

        next_record = storage["daily:10001:2026-05-07"]
        next_stats = storage["stats:10001"]
        self.assertEqual(ctx.results[-1], {"handled": True})
        self.assertNotEqual(next_record["draw_source_fingerprint"], old_record["draw_source_fingerprint"])
        self.assertEqual(next_record["draw_source_fingerprint"], fortune.draw_source_fingerprint(defaults))
        self.assertEqual(next_stats["total_days"], 1)
        self.assertEqual(next_stats["counts"][old_record["fortune"]["name"]], 0)
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
