import os
import sys
import unittest
import json


PLUGIN_DIR = os.path.dirname(os.path.dirname(__file__))
sys.path.insert(0, PLUGIN_DIR)

from bilibili import dynamic_updates
from main import (
    SubscriptionHubPlugin,
    add_bilibili_subscription,
    build_status_text,
    format_subscription_list,
    parse_bilibili_command_args,
    remove_bilibili_subscription,
)
from rendering import build_render_data
from settings import merge_settings


class FakeContext:
    def __init__(self, args=None, target_type="group", target_id="10000", actor=None):
        self.args = args or []
        self.target_type = target_type
        self.target_id = target_id
        self.actor = actor or {"id": "42", "nickname": "订阅人"}
        self.config_writes = []
        self.scheduler_creates = []
        self.texts = []
        self.results = []

    def config_write(self, values):
        self.config_writes.append(values)
        return {"changed_keys": sorted(values.keys())}

    def scheduler_create(self, task_id, cron, payload=None):
        self.scheduler_creates.append({"task_id": task_id, "cron": cron, "payload": payload})
        return {"task_id": task_id}

    def send_text(self, text):
        self.texts.append(text)

    def send_result(self, result):
        self.results.append(result)


class SubscriptionHubTests(unittest.TestCase):
    def test_manifest_declares_visible_command(self):
        with open(os.path.join(PLUGIN_DIR, "info.json"), "r", encoding="utf-8") as handle:
            manifest = json.load(handle)
        self.assertEqual([
            "订阅状态",
            "订阅b站推送",
            "取消b站推送",
            "订阅列表",
            "b站订阅列表",
            "全部订阅列表",
            "全部b站订阅列表",
        ], [item.get("name") for item in manifest.get("commands") or []])
        self.assertEqual("super_admin", manifest["commands"][1]["permission"])
        self.assertEqual("super_admin", manifest["commands"][2]["permission"])
        self.assertEqual("super_admin", manifest["commands"][5]["permission"])
        self.assertEqual("super_admin", manifest["commands"][6]["permission"])
        self.assertNotIn("dynamic_commands", manifest)

    def test_merge_settings_normalizes_tokens_and_subscriptions(self):
        settings = merge_settings({}, {
            "tokens": [{
                "id": "Primary",
                "label": "主 token",
                "secret_key": "bili.primary",
            }],
            "subscriptions": [{
                "uid": "123456",
                "name": "测试 UP",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video", "live", "invalid"],
                "subscribers": [{"id": "42", "nickname": "订阅人"}],
            }],
        })
        self.assertEqual(settings["tokens"][0]["id"], "primary")
        self.assertEqual(settings["subscriptions"][0]["services"], ["video", "live"])
        self.assertEqual(settings["subscriptions"][0]["subscribers"][0]["nickname"], "订阅人")

    def test_dynamic_updates_extract_video(self):
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "987",
                    "basic": {"jump_url": "//www.bilibili.com/video/BV1"},
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_time": "今天 12:00"},
                        "module_dynamic": {
                            "major": {
                                "type": "MAJOR_TYPE_ARCHIVE",
                                "archive": {"title": "新视频", "desc": "视频简介"},
                            },
                        },
                    },
                }],
            },
        })
        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "video")
        self.assertEqual(updates[0]["title"], "新视频")

    def test_dynamic_updates_extract_repost_before_major_type(self):
        updates = dynamic_updates({
            "data": {
                "items": [{
                    "id_str": "988",
                    "basic": {
                        "comment_type": "17",
                        "jump_url": "//t.bilibili.com/988",
                    },
                    "modules": {
                        "module_author": {"name": "测试 UP", "pub_time": "今天 13:00"},
                        "module_dynamic": {
                            "desc": {"text": "转发推荐"},
                            "major": {
                                "type": "MAJOR_TYPE_OPUS",
                                "opus": {"title": "被转发内容"},
                            },
                        },
                    },
                }],
            },
        })
        self.assertEqual(len(updates), 1)
        self.assertEqual(updates[0]["service"], "repost")

    def test_render_data_contains_subscriber_info(self):
        data = build_render_data({
            "uid": "123456",
            "name": "测试 UP",
            "subscribers": [{"id": "42", "nickname": "订阅人"}],
        }, {
            "service": "video",
            "title": "新视频",
            "summary": "视频简介",
            "author": {"name": "测试 UP"},
        })
        self.assertEqual(data["subscriber_text"], "订阅人")
        self.assertEqual(data["service"], "视频")

    def test_parse_bilibili_command_args_defaults_to_all(self):
        self.assertEqual(parse_bilibili_command_args(["123456"]), {"services": ["all"], "uid": "123456", "error": False})
        self.assertEqual(parse_bilibili_command_args(["图文", "123456"]), {"services": ["image_text"], "uid": "123456", "error": False})
        self.assertEqual(parse_bilibili_command_args(["番剧", "123456"]), {"services": [], "uid": "123456", "error": True})

    def test_add_bilibili_subscription_binds_current_target_and_subscriber(self):
        settings = merge_settings({}, {})
        result = add_bilibili_subscription(settings, FakeContext(args=["图文", "123456"]))
        self.assertTrue(result["ok"])
        self.assertEqual(len(settings["subscriptions"]), 1)
        subscription = settings["subscriptions"][0]
        self.assertEqual(subscription["id"], "bilibili-123456-group-10000")
        self.assertEqual(subscription["services"], ["image_text"])
        self.assertEqual(subscription["subscribers"], [{"id": "42", "nickname": "订阅人"}])

    def test_add_bilibili_subscription_merges_services_and_subscribers(self):
        settings = merge_settings({}, {
            "subscriptions": [{
                "uid": "123456",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video"],
                "subscribers": [{"id": "42", "nickname": "旧昵称"}],
            }],
        })
        result = add_bilibili_subscription(settings, FakeContext(args=["直播", "123456"], actor={"id": "43", "nickname": "新订阅人"}))
        self.assertTrue(result["ok"])
        self.assertEqual(settings["subscriptions"][0]["services"], ["video", "live"])
        self.assertEqual(settings["subscriptions"][0]["subscribers"], [
            {"id": "42", "nickname": "旧昵称"},
            {"id": "43", "nickname": "新订阅人"},
        ])

    def test_remove_bilibili_subscription_removes_service_or_item(self):
        settings = merge_settings({}, {
            "subscriptions": [{
                "uid": "123456",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video", "live"],
            }],
        })
        result = remove_bilibili_subscription(settings, FakeContext(args=["直播", "123456"]))
        self.assertTrue(result["ok"])
        self.assertEqual(settings["subscriptions"][0]["services"], ["video"])

        result = remove_bilibili_subscription(settings, FakeContext(args=["123456"]))
        self.assertTrue(result["ok"])
        self.assertEqual(settings["subscriptions"], [])

    def test_format_subscription_list_can_filter_current_target_and_platform(self):
        settings = merge_settings({}, {
            "subscriptions": [
                {"uid": "123456", "target_type": "group", "target_id": "10000", "services": ["video"], "subscribers": ["订阅人"]},
                {"uid": "654321", "target_type": "private", "target_id": "42", "services": ["live"]},
            ],
        })
        text = format_subscription_list(settings, {"target_type": "group", "target_id": "10000"}, platform="bilibili", title="Bilibili 订阅列表")
        self.assertIn("Bilibili 123456", text)
        self.assertNotIn("654321", text)

    def test_subscribe_command_registers_scheduler_after_saving(self):
        plugin = SubscriptionHubPlugin()
        plugin._settings_loaded = True
        plugin._settings = merge_settings(plugin._default_settings, {"poll_cron": "*/7 * * * *"})
        ctx = FakeContext(args=["视频", "123456"])

        plugin.handle_subscribe_bilibili(ctx)

        self.assertEqual(len(ctx.config_writes), 1)
        self.assertEqual(ctx.scheduler_creates, [{
            "task_id": "subscription-hub-poll",
            "cron": "*/7 * * * *",
            "payload": {"kind": "subscription_poll"},
        }])
        self.assertEqual(ctx.results[-1], {"handled": True})

    def test_unsubscribe_command_registers_scheduler_after_saving(self):
        plugin = SubscriptionHubPlugin()
        plugin._settings_loaded = True
        plugin._settings = merge_settings(plugin._default_settings, {
            "subscriptions": [{
                "uid": "123456",
                "target_type": "group",
                "target_id": "10000",
                "services": ["video"],
            }],
        })
        ctx = FakeContext(args=["123456"])

        plugin.handle_unsubscribe_bilibili(ctx)

        self.assertEqual(len(ctx.config_writes), 1)
        self.assertEqual(ctx.scheduler_creates[0]["task_id"], "subscription-hub-poll")
        self.assertEqual(ctx.results[-1], {"handled": True})

    def test_build_status_text_summarizes_visible_state(self):
        settings = merge_settings({}, {
            "enabled": True,
            "poll_cron": "*/10 * * * *",
            "tokens": [
                {"id": "primary", "label": "主 token", "secret_key": "bili.primary", "enabled": True},
                {"id": "backup", "label": "备用 token", "secret_key": "bili.backup", "enabled": False},
            ],
            "subscriptions": [
                {"uid": "123456", "target_type": "group", "target_id": "10000"},
                {"uid": "654321", "target_type": "private", "target_id": "42", "enabled": False},
            ],
        })
        self.assertEqual(build_status_text(settings), "\n".join([
            "订阅中心",
            "状态：启用",
            "订阅：1/2",
            "Token：1/2",
            "轮询：*/10 * * * *",
        ]))


if __name__ == "__main__":
    unittest.main()
