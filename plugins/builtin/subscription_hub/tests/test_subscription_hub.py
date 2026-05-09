import os
import sys
import unittest
import json


PLUGIN_DIR = os.path.dirname(os.path.dirname(__file__))
sys.path.insert(0, PLUGIN_DIR)

from bilibili import dynamic_updates
from main import build_status_text
from rendering import build_render_data
from settings import merge_settings


class SubscriptionHubTests(unittest.TestCase):
    def test_manifest_declares_visible_command(self):
        with open(os.path.join(PLUGIN_DIR, "info.json"), "r", encoding="utf-8") as handle:
            manifest = json.load(handle)
        self.assertEqual([{
            "name": "订阅状态",
            "description": "查看订阅中心状态",
            "usage": "/订阅状态",
            "permission": "everyone",
        }], manifest.get("commands"))
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
